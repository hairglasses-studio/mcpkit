//go:build !official_sdk

// Command stateless-http demonstrates a horizontally scalable MCP server
// with Redis-backed sessions. It shows:
//   - RedisStringStore for session persistence across server instances
//   - SessionExtractor for multi-source session ID extraction (header, cookie, query)
//   - AffinityMiddleware for session-aware request routing
//   - Session middleware that creates or loads sessions per request
//   - Docker Compose setup with nginx round-robin load balancing
//
// The key insight: because sessions live in Redis (not in-process memory),
// any server instance can handle any request. Nginx round-robins freely
// and sessions still work correctly.
//
// Run standalone:
//
//	REDIS_URL=redis://localhost:6379 go run ./examples/stateless-http/
//
// Run with Docker Compose (two instances + nginx + Redis):
//
//	cd examples/stateless-http && docker compose up --build
//
// Then test:
//
//	# Create a session — the server returns a session ID in the cookie.
//	curl -v http://localhost:80/session/new
//
//	# Use the session — note the X-Served-By header changes (round-robin)
//	# but the session data is consistent across instances.
//	curl -b "mcp_session=<id>" http://localhost:80/session/info
//	curl -b "mcp_session=<id>" -X POST http://localhost:80/session/incr
//	curl -b "mcp_session=<id>" http://localhost:80/session/info
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/lifecycle"
	"github.com/hairglasses-studio/mcpkit/logging"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/session"
	"github.com/hairglasses-studio/mcpkit/transport"
)

// ---------------------------------------------------------------------------
// Minimal Redis client using the session.RedisClient interface.
// In production, you would implement this with go-redis, rueidis, etc.
// This in-memory version demonstrates the interface contract without
// pulling in a real Redis dependency.
// ---------------------------------------------------------------------------

// MemRedisClient is a fake Redis client backed by an in-memory map.
// It implements session.RedisClient for demonstration purposes.
// In a real deployment, replace this with a go-redis wrapper:
//
//	type GoRedisClient struct { rdb *redis.Client }
//	func (c *GoRedisClient) Get(ctx context.Context, key string) (string, error) {
//	    val, err := c.rdb.Get(ctx, key).Result()
//	    if err == redis.Nil { return "", session.ErrNotFound }
//	    return val, err
//	}
//	func (c *GoRedisClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
//	    return c.rdb.Set(ctx, key, value, ttl).Err()
//	}
//	func (c *GoRedisClient) Del(ctx context.Context, keys ...string) error {
//	    return c.rdb.Del(ctx, keys...).Err()
//	}
type MemRedisClient struct {
	data map[string]memEntry
}

type memEntry struct {
	value     string
	expiresAt time.Time // zero = no expiry
}

func NewMemRedisClient() *MemRedisClient {
	return &MemRedisClient{data: make(map[string]memEntry)}
}

func (m *MemRedisClient) Get(_ context.Context, key string) (string, error) {
	entry, ok := m.data[key]
	if !ok {
		return "", session.ErrNotFound
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(m.data, key)
		return "", session.ErrNotFound
	}
	return entry.value, nil
}

func (m *MemRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	entry := memEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	m.data[key] = entry
	return nil
}

func (m *MemRedisClient) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tool types — session-aware tools that demonstrate cross-instance state
// ---------------------------------------------------------------------------

// CounterInput is the input for the session_counter tool.
type CounterInput struct {
	Action string `json:"action" jsonschema:"required,enum=get|increment|reset,description=Action to perform on the session counter"`
}

// CounterOutput is the output for the session_counter tool.
type CounterOutput struct {
	Counter   int    `json:"counter"`
	SessionID string `json:"session_id"`
	ServerID  string `json:"server_id"`
	Action    string `json:"action"`
}

// NoteInput is the input for the session_note tool.
type NoteInput struct {
	Action string `json:"action" jsonschema:"required,enum=set|get|clear,description=Action to perform on the session note"`
	Text   string `json:"text,omitempty" jsonschema:"description=Note text (required for set action)"`
}

// NoteOutput is the output for the session_note tool.
type NoteOutput struct {
	Note      string `json:"note"`
	SessionID string `json:"session_id"`
	ServerID  string `json:"server_id"`
	Action    string `json:"action"`
}

// WhoAmIInput is the input for the whoami tool (no parameters).
type WhoAmIInput struct{}

// WhoAmIOutput is the output for the whoami tool.
type WhoAmIOutput struct {
	SessionID string `json:"session_id"`
	ServerID  string `json:"server_id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

// SessionDemoModule provides session-aware demo tools.
type SessionDemoModule struct {
	serverID string
	store    session.SessionStore
}

func (m *SessionDemoModule) Name() string        { return "session_demo" }
func (m *SessionDemoModule) Description() string { return "Session-aware demo tools for stateless HTTP scaling" }

func (m *SessionDemoModule) Tools() []registry.ToolDefinition {
	serverID := m.serverID

	// session_counter: increment/get/reset a per-session counter.
	// The counter persists in Redis, so any server instance can read/write it.
	counterTool := handler.TypedHandler[CounterInput, CounterOutput](
		"session_counter",
		"Manage a per-session counter stored in Redis. Demonstrates that state persists across server instances behind a load balancer.",
		func(ctx context.Context, input CounterInput) (CounterOutput, error) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return CounterOutput{}, fmt.Errorf("no session in context — ensure session middleware is active")
			}

			var counter int
			if v, found := sess.Get("counter"); found {
				if c, ok := v.(float64); ok {
					counter = int(c)
				}
			}

			switch input.Action {
			case "increment":
				counter++
				sess.Set("counter", float64(counter))
				if err := m.store.(*session.RedisStringStore).Save(ctx, sess); err != nil {
					return CounterOutput{}, fmt.Errorf("save session: %w", err)
				}
			case "reset":
				counter = 0
				sess.Set("counter", float64(counter))
				if err := m.store.(*session.RedisStringStore).Save(ctx, sess); err != nil {
					return CounterOutput{}, fmt.Errorf("save session: %w", err)
				}
			case "get":
				// read-only, no save needed
			default:
				return CounterOutput{}, fmt.Errorf("unknown action %q: use get, increment, or reset", input.Action)
			}

			return CounterOutput{
				Counter:   counter,
				SessionID: sess.ID(),
				ServerID:  serverID,
				Action:    input.Action,
			}, nil
		},
	)
	counterTool.Category = "session"
	counterTool.Tags = []string{"session", "counter", "demo"}
	counterTool.Complexity = registry.ComplexitySimple

	// session_note: set/get/clear a per-session note.
	noteTool := handler.TypedHandler[NoteInput, NoteOutput](
		"session_note",
		"Manage a per-session text note stored in Redis. Set, get, or clear a note that follows the session across server instances.",
		func(ctx context.Context, input NoteInput) (NoteOutput, error) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return NoteOutput{}, fmt.Errorf("no session in context — ensure session middleware is active")
			}

			var note string
			if v, found := sess.Get("note"); found {
				if s, ok := v.(string); ok {
					note = s
				}
			}

			switch input.Action {
			case "set":
				if input.Text == "" {
					return NoteOutput{}, fmt.Errorf("text is required for set action")
				}
				note = input.Text
				sess.Set("note", note)
				if err := m.store.(*session.RedisStringStore).Save(ctx, sess); err != nil {
					return NoteOutput{}, fmt.Errorf("save session: %w", err)
				}
			case "clear":
				note = ""
				sess.Delete("note")
				if err := m.store.(*session.RedisStringStore).Save(ctx, sess); err != nil {
					return NoteOutput{}, fmt.Errorf("save session: %w", err)
				}
			case "get":
				// read-only
			default:
				return NoteOutput{}, fmt.Errorf("unknown action %q: use set, get, or clear", input.Action)
			}

			return NoteOutput{
				Note:      note,
				SessionID: sess.ID(),
				ServerID:  serverID,
				Action:    input.Action,
			}, nil
		},
	)
	noteTool.Category = "session"
	noteTool.Tags = []string{"session", "note", "demo"}
	noteTool.Complexity = registry.ComplexitySimple

	// whoami: return session + server identity. No mutation, just identity.
	whoamiTool := handler.TypedHandler[WhoAmIInput, WhoAmIOutput](
		"whoami",
		"Return the current session ID and which server instance is handling the request. Useful for verifying round-robin load balancing.",
		func(ctx context.Context, _ WhoAmIInput) (WhoAmIOutput, error) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return WhoAmIOutput{
					ServerID: serverID,
					Message:  "no session — request was not routed through session middleware",
				}, nil
			}
			return WhoAmIOutput{
				SessionID: sess.ID(),
				ServerID:  serverID,
				CreatedAt: sess.CreatedAt().Format(time.RFC3339),
				Message:   fmt.Sprintf("handled by %s", serverID),
			}, nil
		},
	)
	whoamiTool.Category = "session"
	whoamiTool.Tags = []string{"session", "identity", "debug"}
	whoamiTool.Complexity = registry.ComplexitySimple

	return []registry.ToolDefinition{counterTool, noteTool, whoamiTool}
}

// ---------------------------------------------------------------------------
// REST endpoints for easy curl testing (in addition to MCP)
// ---------------------------------------------------------------------------

func restRoutes(mux *http.ServeMux, store *session.RedisStringStore, extractor *transport.SessionExtractor, serverID string) {
	// POST /session/new — create a new session, return the ID in a cookie.
	mux.HandleFunc("POST /session/new", func(w http.ResponseWriter, r *http.Request) {
		sess, err := store.Create(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		extractor.SetCookie(w, sess.ID())
		w.Header().Set("X-Served-By", serverID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": sess.ID(),
			"server_id":  serverID,
			"message":    "session created",
		})
	})

	// GET /session/new — also accept GET for easier curl testing.
	mux.HandleFunc("GET /session/new", func(w http.ResponseWriter, r *http.Request) {
		sess, err := store.Create(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		extractor.SetCookie(w, sess.ID())
		w.Header().Set("X-Served-By", serverID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"session_id": sess.ID(),
			"server_id":  serverID,
			"message":    "session created",
		})
	})

	// GET /session/info — return session metadata + data.
	mux.HandleFunc("GET /session/info", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := session.FromContext(r.Context())
		if !ok {
			w.Header().Set("X-Served-By", serverID)
			http.Error(w, `{"error":"no session — use /session/new first"}`, http.StatusUnauthorized)
			return
		}
		counter := 0.0
		if v, found := sess.Get("counter"); found {
			if c, ok := v.(float64); ok {
				counter = c
			}
		}
		note := ""
		if v, found := sess.Get("note"); found {
			if s, ok := v.(string); ok {
				note = s
			}
		}
		w.Header().Set("X-Served-By", serverID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": sess.ID(),
			"server_id":  serverID,
			"created_at": sess.CreatedAt().Format(time.RFC3339),
			"counter":    int(counter),
			"note":       note,
		})
	})

	// POST /session/incr — increment the counter.
	mux.HandleFunc("POST /session/incr", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := session.FromContext(r.Context())
		if !ok {
			w.Header().Set("X-Served-By", serverID)
			http.Error(w, `{"error":"no session — use /session/new first"}`, http.StatusUnauthorized)
			return
		}
		counter := 0.0
		if v, found := sess.Get("counter"); found {
			if c, ok := v.(float64); ok {
				counter = c
			}
		}
		counter++
		sess.Set("counter", counter)
		if err := store.Save(r.Context(), sess); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Served-By", serverID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": sess.ID(),
			"server_id":  serverID,
			"counter":    int(counter),
		})
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	ctx := context.Background()
	logger := slog.Default()

	// --- Configuration from environment ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serverID := os.Getenv("SERVER_ID")
	if serverID == "" {
		hostname, _ := os.Hostname()
		serverID = fmt.Sprintf("mcp-%s-%s", hostname, port)
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		log.Printf("REDIS_URL=%s (would connect in production; using in-memory mock for this example)", redisURL)
	}

	// --- Redis-backed session store ---
	// In production, replace MemRedisClient with a real Redis client.
	// See the GoRedisClient example in the MemRedisClient docs above.
	redisClient := NewMemRedisClient()
	store := session.NewRedisStringStore(redisClient,
		session.WithPrefix("mcp:session:"),
		session.WithTTL(30*time.Minute),
	)

	log.Printf("session store: RedisStringStore (prefix=%q, ttl=30m)", "mcp:session:")

	// --- Session extractor ---
	// Checks Authorization header, X-Session-ID header, cookie, query param
	// in priority order.
	extractor := transport.NewSessionExtractor(transport.SessionExtractorConfig{
		HeaderName:   "X-Session-ID",
		CookieName:   "mcp_session",
		QueryParam:   "session_id",
		CookieSecure: false, // false for local development
		CookieTTL:    24 * time.Hour,
	})

	// --- Tool registry ---
	reg := registry.NewToolRegistry(registry.Config{
		DefaultTimeout: 30 * time.Second,
		Middleware: []registry.Middleware{
			logging.Middleware(logger),
		},
	})
	reg.RegisterModule(&SessionDemoModule{
		serverID: serverID,
		store:    store,
	})

	// --- Health checker ---
	checker := health.NewChecker(
		health.WithToolCount(reg.ToolCount),
	)

	// --- MCP server ---
	mcpServer := server.NewMCPServer(
		"stateless-http-example",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	reg.RegisterWithServer(mcpServer)

	// --- StreamableHTTP transport ---
	httpTransport := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	// --- HTTP mux ---
	mux := http.NewServeMux()

	// Session middleware: extracts session from cookie/header/query, loads
	// from Redis, attaches to context. New sessions must be created
	// explicitly via /session/new.
	sessionMW := session.TokenMiddleware(store, session.TokenMiddlewareOptions{
		Header:     "X-Session-ID",
		CookieName: "mcp_session",
		QueryParam: "session_id",
	})

	// MCP endpoint with session middleware.
	mux.Handle("/mcp", sessionMW(httpTransport))

	// REST endpoints for easy curl testing.
	restMux := http.NewServeMux()
	restRoutes(restMux, store, extractor, serverID)
	mux.Handle("/session/", sessionMW(restMux))

	// Health endpoints.
	healthHandler := health.Handler(checker)
	mux.Handle("/health", healthHandler)
	mux.Handle("/ready", healthHandler)
	mux.Handle("/live", healthHandler)

	// Server ID header on all responses.
	wrappedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Served-By", serverID)
		mux.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      wrappedMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Lifecycle manager ---
	lm := lifecycle.New(lifecycle.Config{
		DrainTimeout: 15 * time.Second,
		OnHealthy: func() {
			checker.SetStatus("healthy")
			log.Printf("%s: listening on http://localhost:%s", serverID, port)
			log.Printf("%s: MCP endpoint at /mcp, REST at /session/*", serverID)
		},
		OnDraining: func() {
			checker.SetStatus("draining")
			log.Printf("%s: draining", serverID)
		},
	})

	lm.OnShutdown(func(ctx context.Context) error {
		log.Printf("%s: shutting down", serverID)
		return httpServer.Shutdown(ctx)
	})

	if err := lm.Run(ctx, func(ctx context.Context) error {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}
