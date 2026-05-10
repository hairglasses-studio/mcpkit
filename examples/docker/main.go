//go:build !official_sdk

// Command docker demonstrates a container-ready StreamableHTTP MCP server.
// It shows:
//   - Docker Compose deployment with two MCP server instances behind nginx
//   - Health endpoints for container health checks and load balancer readiness
//   - Lifecycle manager integration for graceful SIGTERM drain
//   - Server identity metadata so load balancing is easy to verify
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/discovery"
	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/lifecycle"
	"github.com/hairglasses-studio/mcpkit/logging"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type EchoInput struct {
	Message string `json:"message" jsonschema:"required,description=Message to echo back"`
}

type EchoOutput struct {
	Echo      string `json:"echo"`
	ServerID  string `json:"server_id"`
	Timestamp string `json:"timestamp"`
}

type StatusInput struct{}

type StatusOutput struct {
	ServerID  string `json:"server_id"`
	Hostname  string `json:"hostname"`
	Uptime    string `json:"uptime"`
	StartedAt string `json:"started_at"`
}

type ContainerModule struct {
	serverID string
	hostname string
	started  time.Time
}

func (m *ContainerModule) Name() string        { return "container" }
func (m *ContainerModule) Description() string { return "Container deployment smoke-test tools" }

func (m *ContainerModule) Tools() []registry.ToolDefinition {
	echoTool := handler.TypedHandler[EchoInput, EchoOutput](
		"docker_echo",
		"Echo a message with the serving container identity.",
		func(_ context.Context, input EchoInput) (EchoOutput, error) {
			if input.Message == "" {
				return EchoOutput{}, fmt.Errorf("message must not be empty")
			}
			return EchoOutput{
				Echo:      input.Message,
				ServerID:  m.serverID,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}, nil
		},
	)
	echoTool.Category = "docker"
	echoTool.Tags = []string{"docker", "echo", "smoke-test"}
	echoTool.Complexity = registry.ComplexitySimple

	statusTool := handler.TypedHandler[StatusInput, StatusOutput](
		"docker_status",
		"Return container identity and uptime for deployment smoke tests.",
		func(_ context.Context, _ StatusInput) (StatusOutput, error) {
			return StatusOutput{
				ServerID:  m.serverID,
				Hostname:  m.hostname,
				Uptime:    time.Since(m.started).Truncate(time.Second).String(),
				StartedAt: m.started.UTC().Format(time.RFC3339),
			}, nil
		},
	)
	statusTool.Category = "docker"
	statusTool.Tags = []string{"docker", "health", "read-only"}
	statusTool.Complexity = registry.ComplexitySimple

	return []registry.ToolDefinition{echoTool, statusTool}
}

func main() {
	ctx := context.Background()
	started := time.Now()
	serverID := env("SERVER_ID", "docker-example")
	port := env("PORT", "8080")
	addr := ":" + port
	publicBaseURL := env("PUBLIC_BASE_URL", "http://localhost:"+port)

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = serverID
	}

	var activeTasks atomic.Int64
	reg := registry.NewToolRegistry(registry.Config{
		DefaultTimeout: 30 * time.Second,
		Middleware: []registry.Middleware{
			activeTaskMiddleware(&activeTasks),
			logging.Middleware(slog.Default()),
		},
	})
	reg.RegisterModule(&ContainerModule{
		serverID: serverID,
		hostname: hostname,
		started:  started,
	})

	checker := health.NewChecker(
		health.WithToolCount(reg.ToolCount),
		health.WithTaskCount(func() int { return int(activeTasks.Load()) }),
		health.WithCustomChecker("container", func(context.Context) any {
			return map[string]string{
				"server_id": serverID,
				"hostname":  hostname,
			}
		}),
	)
	checker.SetStatus("starting")

	mcpServer := server.NewMCPServer(
		"docker-example",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	reg.RegisterWithServer(mcpServer)

	httpTransport := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	cardCfg := discovery.MetadataConfig{
		Name:         "io.github.hairglasses-studio.docker-example",
		Description:  "Container-ready MCP server demonstrating Docker health checks and lifecycle drain",
		Version:      "1.0.0",
		Organization: "hairglasses-studio",
		Repository:   "https://github.com/hairglasses-studio/mcpkit",
		Homepage:     "https://github.com/hairglasses-studio/mcpkit/tree/main/examples/docker",
		License:      "MIT",
		Categories:   []string{"developer-tools", "examples"},
		Tags:         []string{"mcpkit", "docker", "compose", "health"},
		Transports:   []discovery.TransportInfo{{Type: "streamable-http", URL: publicBaseURL + "/mcp"}},
		Tools:        reg,
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", httpTransport)
	mux.Handle("/health", health.Handler(checker))
	mux.Handle("/ready", health.Handler(checker))
	mux.Handle("/live", health.Handler(checker))
	mux.Handle("/.well-known/mcp.json", discovery.ServerCardHandler(cardCfg))
	mux.HandleFunc("/identity", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{
			"server_id":  serverID,
			"hostname":   hostname,
			"started_at": started.UTC().Format(time.RFC3339),
		})
	})

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	lm := lifecycle.New(lifecycle.Config{
		DrainTimeout: 15 * time.Second,
		OnHealthy: func() {
			checker.SetStatus("healthy")
			log.Printf("docker-example[%s]: listening on %s", serverID, addr)
		},
		OnDraining: func() {
			checker.SetStatus("draining")
			log.Printf("docker-example[%s]: draining", serverID)
		},
	})
	lm.OnShutdown(func(ctx context.Context) error {
		checker.SetStatus("stopped")
		return httpServer.Shutdown(ctx)
	})

	if err := lm.Run(ctx, func(context.Context) error {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func activeTaskMiddleware(active *atomic.Int64) registry.Middleware {
	return func(_ string, _ registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			active.Add(1)
			defer active.Add(-1)
			return next(ctx, request)
		}
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
