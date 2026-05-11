//go:build !official_sdk

package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

func TestServeOptionsApply(t *testing.T) {
	t.Parallel()

	mw := func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			return next(ctx, req)
		}
	}
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	resourceRegistry := resources.NewResourceRegistry()
	promptRegistry := prompts.NewPromptRegistry()
	moduleA := &testToolModule{}
	moduleB := &struct{ testToolModule }{}
	serverOpt := server.WithInstructions("test instructions")
	registryConfig := registry.Config{
		DefaultTimeout:  2 * time.Second,
		MaxResponseSize: 2048,
		ToolNamePrefix:  "test_",
	}

	opts := defaultServeOptions()
	if opts.logLevel != slog.LevelInfo {
		t.Fatalf("default log level = %v, want info", opts.logLevel)
	}

	for _, opt := range []Option{
		WithModule(moduleA),
		WithModules(moduleB),
		WithMiddleware(mw),
		WithRegistryConfig(registryConfig),
		WithAudit("/tmp/bootstrap-audit.jsonl"),
		WithSafetyTiers(),
		WithLogHandler(handler),
		WithLogLevel(slog.LevelDebug),
		WithResources(resourceRegistry),
		WithPrompts(promptRegistry),
		WithServerOption(serverOpt),
	} {
		opt(&opts)
	}

	if len(opts.modules) != 2 || opts.modules[0] != moduleA || opts.modules[1] != moduleB {
		t.Fatalf("modules = %#v, want moduleA,moduleB", opts.modules)
	}
	if len(opts.middleware) != 1 || opts.middleware[0] == nil {
		t.Fatalf("middleware = %#v, want one middleware", opts.middleware)
	}
	if opts.registryConfig.DefaultTimeout != registryConfig.DefaultTimeout ||
		opts.registryConfig.MaxResponseSize != registryConfig.MaxResponseSize ||
		opts.registryConfig.ToolNamePrefix != registryConfig.ToolNamePrefix {
		t.Fatalf("registry config = %+v, want %+v", opts.registryConfig, registryConfig)
	}
	if opts.auditPath != "/tmp/bootstrap-audit.jsonl" {
		t.Fatalf("audit path = %q", opts.auditPath)
	}
	if !opts.safetyTiers {
		t.Fatal("safety tiers were not enabled")
	}
	if opts.logHandler != handler {
		t.Fatal("custom log handler was not stored")
	}
	if opts.logLevel != slog.LevelDebug {
		t.Fatalf("log level = %v, want debug", opts.logLevel)
	}
	if opts.resourceRegistry != resourceRegistry {
		t.Fatal("resource registry was not stored")
	}
	if opts.promptRegistry != promptRegistry {
		t.Fatal("prompt registry was not stored")
	}
	if len(opts.serverOpts) != 1 || opts.serverOpts[0] == nil {
		t.Fatalf("server opts = %#v, want one server option", opts.serverOpts)
	}
}

func TestServeBuildsServerAndDelegates(t *testing.T) {
	oldServeAuto := serveAuto
	defer func() { serveAuto = oldServeAuto }()

	sentinel := errors.New("serve stopped")
	called := false
	serveAuto = func(s *registry.MCPServer) error {
		called = true
		if s == nil {
			t.Fatal("Serve delegated a nil server")
		}
		return sentinel
	}

	resourceRegistry := resources.NewResourceRegistry()
	resourceRegistry.RegisterModule(&testResourceModule{})
	promptRegistry := prompts.NewPromptRegistry()
	promptRegistry.RegisterModule(&testPromptModule{})

	mw := func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			return next(ctx, req)
		}
	}

	err := Serve(
		ServerConfig{Name: "unit-server", Version: "v1"},
		WithModule(&testToolModule{}),
		WithAudit(t.TempDir()+"/audit.jsonl"),
		WithSafetyTiers(),
		WithMiddleware(mw),
		WithRegistryConfig(registry.Config{MaxResponseSize: 4096}),
		WithResources(resourceRegistry),
		WithPrompts(promptRegistry),
		WithServerOption(server.WithInstructions("unit test")),
	)

	if !errors.Is(err, sentinel) {
		t.Fatalf("Serve() error = %v, want %v", err, sentinel)
	}
	if !called {
		t.Fatal("Serve() did not delegate to serveAuto")
	}
}
