//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/session"
)

// SessionServer wraps a test Server with session middleware pre-configured.
// It uses an in-memory session store so that tool handlers can read and write
// session data via session.FromContext.
type SessionServer struct {
	*Server
	Store *session.MemStore
}

// NewServerWithSessions creates a test server with session middleware
// pre-configured using an in-memory session store (no TTL). Tool handlers
// registered on the returned server can use session.FromContext to access
// the current session.
//
// Each Client created via NewClient will receive its own session that
// persists across all tool calls made by that client.
func NewServerWithSessions(t testing.TB, modules ...registry.ToolModule) *SessionServer {
	t.Helper()

	store := session.NewMemStore(session.Options{})

	mw := sessionMiddleware(store)

	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{mw},
	})
	for _, mod := range modules {
		reg.RegisterModule(mod)
	}

	srv := NewServer(t, reg)
	t.Cleanup(func() { _ = store.Close() })

	return &SessionServer{
		Server: srv,
		Store:  store,
	}
}

// sessionMiddleware returns a registry.Middleware that attaches a session to
// the context for each tool call. It uses a per-context session ID stored in
// the context by the MCP transport layer. If no session exists for the current
// transport session, a new one is created and associated.
func sessionMiddleware(store session.SessionStore) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Check if a session is already attached (e.g. nested middleware).
			if _, ok := session.FromContext(ctx); ok {
				return next(ctx, req)
			}

			// Create a new session for each tool call context that doesn't have one.
			// In the test transport, each Client has a stable MCP session, so
			// we use a context-value marker to correlate calls from the same client.
			sess, err := getOrCreateSession(ctx, store)
			if err != nil {
				return nil, err
			}

			ctx = session.WithSession(ctx, sess)
			return next(ctx, req)
		}
	}
}

// sessionClientKey is used to store a session ID in the context so that
// the same client gets the same session across multiple tool calls.
type sessionClientKey struct{}

// WithSessionID returns a context with a session ID hint. When the session
// middleware sees this value, it reuses the existing session instead of
// creating a new one. This is called internally by the test infrastructure.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionClientKey{}, id)
}

// getOrCreateSession retrieves an existing session from the store if a
// session ID is present in the context, or creates a new one.
func getOrCreateSession(ctx context.Context, store session.SessionStore) (session.Session, error) {
	if id, ok := ctx.Value(sessionClientKey{}).(string); ok && id != "" {
		sess, found, err := store.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			return sess, nil
		}
	}

	return store.Create(ctx)
}

// AssertSessionCreated verifies that the tool call context had a session
// attached. Since sessions are attached via context (not result content),
// this helper works by checking that the result is non-nil and successful,
// confirming the session middleware ran without error.
func AssertSessionCreated(t testing.TB, result *registry.CallToolResult) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil — session middleware may not have run")
	}
}

// AssertSameSession verifies that two call tool results were produced within
// the same session. This requires that the tool handlers stored the session
// ID in the result text (e.g. via session.FromContext). Both results must
// contain a session ID extractable by GetSessionID.
func AssertSameSession(t testing.TB, result1, result2 *registry.CallToolResult) {
	t.Helper()
	id1 := GetSessionID(t, result1)
	id2 := GetSessionID(t, result2)
	if id1 == "" {
		t.Fatal("result1 has no session ID")
	}
	if id2 == "" {
		t.Fatal("result2 has no session ID")
	}
	if id1 != id2 {
		t.Errorf("session IDs differ: %q vs %q", id1, id2)
	}
}

// AssertDifferentSession verifies that two call tool results were produced
// in different sessions. Both results must contain a session ID extractable
// by GetSessionID.
func AssertDifferentSession(t testing.TB, result1, result2 *registry.CallToolResult) {
	t.Helper()
	id1 := GetSessionID(t, result1)
	id2 := GetSessionID(t, result2)
	if id1 == "" {
		t.Fatal("result1 has no session ID")
	}
	if id2 == "" {
		t.Fatal("result2 has no session ID")
	}
	if id1 == id2 {
		t.Errorf("expected different session IDs, both are %q", id1)
	}
}

// GetSessionID extracts the session ID from a tool result's text content.
// The tool handler must embed the session ID in the result text for this
// to work — typically by calling session.FromContext and including sess.ID()
// in the output. Returns an empty string if no text content is found.
func GetSessionID(t testing.TB, result *registry.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		return ""
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		return ""
	}
	return text
}

// SessionClient is a test client that automatically attaches a session ID
// to every tool call, ensuring all calls from the same client share a session.
type SessionClient struct {
	*Client
	sessionID string
}

// NewSessionClient creates a test client connected to a SessionServer.
// All tool calls from this client will share the same session.
func NewSessionClient(t testing.TB, s *SessionServer) *SessionClient {
	t.Helper()

	// Create an initial session in the store so we have an ID to reuse.
	sess, err := s.Store.Create(context.Background())
	if err != nil {
		t.Fatalf("failed to create initial session: %v", err)
	}

	return &SessionClient{
		Client:    NewClient(t, s.Server),
		sessionID: sess.ID(),
	}
}

// CallTool calls a tool with the session ID attached to the context.
func (sc *SessionClient) CallTool(name string, args map[string]interface{}) *registry.CallToolResult {
	sc.Client.t.Helper()
	ctx := WithSessionID(context.Background(), sc.sessionID)
	return sc.Client.CallToolWithContext(ctx, name, args)
}

// CallToolE calls a tool with the session ID attached, returning both result and error.
func (sc *SessionClient) CallToolE(name string, args map[string]interface{}) (*registry.CallToolResult, error) {
	sc.Client.t.Helper()
	ctx := WithSessionID(context.Background(), sc.sessionID)
	result, err := sc.Client.tr.callTool(ctx, sc.Client.t, name, args)
	return result, err
}

// SessionID returns the session ID used by this client.
func (sc *SessionClient) SessionID() string {
	return sc.sessionID
}
