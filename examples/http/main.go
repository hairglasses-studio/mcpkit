//go:build !official_sdk

// Command http demonstrates a production-grade mcpkit StreamableHTTP server.
// It shows:
//   - Tool registration via TypedHandler with auto-generated schemas
//   - StreamableHTTP transport served on port 8080 at /mcp
//   - Health check endpoints (/health, /ready, /live) on the same mux
//   - Logging middleware for tool invocation observability
//   - Lifecycle manager for signal-driven graceful shutdown
//
// Run:
//
//	go run ./examples/http/
//
// Then send MCP requests to http://localhost:8080/mcp.
// Health status is available at http://localhost:8080/health.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/lifecycle"
	"github.com/hairglasses-studio/mcpkit/logging"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ---------------------------------------------------------------------------
// Tool types
// ---------------------------------------------------------------------------

// EchoInput is the input schema for the echo tool.
type EchoInput struct {
	Message string `json:"message" jsonschema:"required,description=The message to echo back"`
}

// EchoOutput is the output schema for the echo tool.
type EchoOutput struct {
	Echo      string `json:"echo"`
	Length    int    `json:"length"`
	Timestamp string `json:"timestamp"`
}

// TimeInput is the input schema for the current_time tool.
type TimeInput struct {
	// Format is optional; defaults to RFC3339 when empty.
	Format string `json:"format,omitempty" jsonschema:"description=Go time format string (default: RFC3339)"`
}

// TimeOutput is the output schema for the current_time tool.
type TimeOutput struct {
	Time   string `json:"time"`
	Unix   int64  `json:"unix"`
	Format string `json:"format"`
}

// ---------------------------------------------------------------------------
// Tool module
// ---------------------------------------------------------------------------

// UtilModule provides utility tools.
type UtilModule struct{}

func (m *UtilModule) Name() string        { return "util" }
func (m *UtilModule) Description() string { return "General utility tools" }

func (m *UtilModule) Tools() []registry.ToolDefinition {
	// echo tool: reflects the caller's message back with metadata.
	echoTool := handler.TypedHandler[EchoInput, EchoOutput](
		"echo",
		"Echo a message back. Returns the original message, its character count, and the server timestamp.",
		func(_ context.Context, input EchoInput) (EchoOutput, error) {
			if input.Message == "" {
				return EchoOutput{}, fmt.Errorf("message must not be empty")
			}
			return EchoOutput{
				Echo:      input.Message,
				Length:    len(input.Message),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		},
	)
	echoTool.Category = "util"
	echoTool.Tags = []string{"echo", "debug"}
	echoTool.Complexity = registry.ComplexitySimple

	// current_time tool: returns the server's current time in a requested format.
	timeTool := handler.TypedHandler[TimeInput, TimeOutput](
		"current_time",
		"Return the server's current UTC time. Optionally accepts a Go time format string (e.g. \"2006-01-02\"). Defaults to RFC3339.",
		func(_ context.Context, input TimeInput) (TimeOutput, error) {
			format := input.Format
			if format == "" {
				format = time.RFC3339
			}
			now := time.Now().UTC()
			formatted := now.Format(format)
			return TimeOutput{
				Time:   formatted,
				Unix:   now.Unix(),
				Format: format,
			}, nil
		},
	)
	timeTool.Category = "util"
	timeTool.Tags = []string{"time", "read-only"}
	timeTool.Complexity = registry.ComplexitySimple

	return []registry.ToolDefinition{echoTool, timeTool}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	ctx := context.Background()
	logger := slog.Default()

	// --- Tool registry with logging middleware ---
	reg := registry.NewToolRegistry(registry.Config{
		DefaultTimeout: 30 * time.Second,
		Middleware: []registry.Middleware{
			logging.Middleware(logger),
		},
	})
	reg.RegisterModule(&UtilModule{})

	// --- Health checker ---
	// Wired into the lifecycle so it reflects drain/stop transitions.
	checker := health.NewChecker(
		health.WithToolCount(reg.ToolCount),
	)

	// --- MCP server ---
	mcpServer := server.NewMCPServer(
		"http-example",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	reg.RegisterWithServer(mcpServer)

	// --- StreamableHTTP transport ---
	// NewStreamableHTTPServer wraps the MCP server and implements http.Handler.
	// It handles POST (requests/notifications), GET (SSE streaming), and DELETE
	// (session teardown) at the configured endpoint path.
	httpTransport := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	// --- HTTP mux: mount MCP transport + health endpoints together ---
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpTransport)

	// Health endpoints: /health (status+uptime), /ready (503 while draining), /live.
	healthHandler := health.Handler(checker)
	mux.Handle("/health", healthHandler)
	mux.Handle("/ready", healthHandler)
	mux.Handle("/live", healthHandler)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Lifecycle manager ---
	// Transitions: starting → healthy → draining → stopped.
	// On SIGTERM/SIGINT it cancels the serve context, drains, then stops.
	lm := lifecycle.New(lifecycle.Config{
		DrainTimeout: 15 * time.Second,
		OnHealthy: func() {
			checker.SetStatus("healthy")
			log.Printf("http-example: listening on http://localhost:8080/mcp")
		},
		OnDraining: func() {
			checker.SetStatus("draining")
			log.Println("http-example: draining — no new connections")
		},
	})

	// Gracefully shut down the HTTP server during drain.
	lm.OnShutdown(func(ctx context.Context) error {
		log.Println("http-example: shutting down HTTP server")
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
