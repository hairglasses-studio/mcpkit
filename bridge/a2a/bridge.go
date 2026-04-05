package a2a

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultAddr is the default listen address for the Bridge HTTP server.
const DefaultAddr = ":8080"

// DefaultShutdownTimeout is the maximum duration to wait for graceful shutdown.
const DefaultShutdownTimeout = 10 * time.Second

// BridgeConfig configures the MCP-to-A2A Bridge.
type BridgeConfig struct {
	// Name is the human-readable name for the A2A agent.
	Name string

	// Description describes the agent's purpose.
	Description string

	// Version is the semantic version string. Default: "1.0.0".
	Version string

	// URL is the base URL where the agent is served (used in the agent card).
	URL string

	// Addr is the listen address for the HTTP server. Default: ":8080".
	Addr string

	// Timeout is the maximum duration for a single tool execution.
	// Default: 30s (DefaultTaskTimeout).
	Timeout time.Duration

	// Logger for bridge operations. If nil, slog.Default() is used.
	Logger *slog.Logger

	// Middleware are mcpkit middleware applied to tool invocations through
	// the bridge.
	Middleware []registry.Middleware
}

// Bridge wires an mcpkit ToolRegistry into a running A2A server. It creates
// a Translator, AgentCardGenerator, and BridgeExecutor, then exposes them
// via an HTTP handler compatible with the A2A protocol.
type Bridge struct {
	registry    *registry.ToolRegistry
	translator  *Translator
	cardGen     *AgentCardGenerator
	executor    *BridgeExecutor
	httpHandler http.Handler
	config      BridgeConfig
	logger      *slog.Logger

	mu     sync.Mutex
	server *http.Server
}

// NewBridge creates a Bridge that translates mcpkit tools into A2A skills.
// The registry must not be nil.
func NewBridge(reg *registry.ToolRegistry, cfg BridgeConfig) (*Bridge, error) {
	if reg == nil {
		return nil, errors.New("a2a: registry must not be nil")
	}

	// Apply defaults.
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Addr == "" {
		cfg.Addr = DefaultAddr
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTaskTimeout
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Build the shared translator.
	translator := &Translator{}

	// Build the agent card generator.
	cardGen := NewAgentCardGenerator(reg, translator, CardConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Version:     cfg.Version,
		URL:         cfg.URL,
	})

	// Build the executor.
	executor := NewBridgeExecutor(reg, ExecutorConfig{
		Translator:  translator,
		Logger:      logger,
		Middleware:   cfg.Middleware,
		TaskTimeout: cfg.Timeout,
	})

	// Build the a2a-go request handler and HTTP handler.
	reqHandler := a2asrv.NewHandler(executor)
	jsonrpcHandler := a2asrv.NewJSONRPCHandler(reqHandler)

	// Build the agent card for the well-known endpoint.
	card := cardGen.Generate()

	// Assemble the HTTP mux: agent card at well-known path, JSON-RPC at root.
	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.Handle("/", jsonrpcHandler)

	return &Bridge{
		registry:    reg,
		translator:  translator,
		cardGen:     cardGen,
		executor:    executor,
		httpHandler: mux,
		config:      cfg,
		logger:      logger,
	}, nil
}

// Handler returns the A2A HTTP handler for embedding in a custom HTTP server
// or test harness. The handler serves both the well-known agent card endpoint
// and the JSON-RPC A2A endpoint.
func (b *Bridge) Handler() http.Handler {
	return b.httpHandler
}

// AgentCard returns the current agent card with skills derived from the registry.
func (b *Bridge) AgentCard() a2atypes.AgentCard {
	card := b.cardGen.Card()
	if card == nil {
		return a2atypes.AgentCard{}
	}
	return *card
}

// Start launches the HTTP server on the configured address. It blocks until
// the server stops or the context is canceled. Use Stop for graceful shutdown.
func (b *Bridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.server != nil {
		b.mu.Unlock()
		return errors.New("a2a: bridge already started")
	}

	srv := &http.Server{
		Addr:    b.config.Addr,
		Handler: b.httpHandler,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	b.server = srv
	b.mu.Unlock()

	b.logger.Info("a2a bridge starting",
		"addr", b.config.Addr,
		"name", b.config.Name,
		"version", b.config.Version,
	)

	// Shut down when context is canceled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			b.logger.Error("a2a bridge shutdown error", "error", err)
		}
	}()

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("a2a: server error: %w", err)
}

// Stop gracefully shuts down the HTTP server. If the server is not running,
// Stop returns nil.
func (b *Bridge) Stop(ctx context.Context) error {
	b.mu.Lock()
	srv := b.server
	b.server = nil
	b.mu.Unlock()

	if srv == nil {
		return nil
	}

	b.logger.Info("a2a bridge stopping")
	return srv.Shutdown(ctx)
}
