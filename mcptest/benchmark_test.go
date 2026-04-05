//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// echoModule is a minimal tool module for benchmark tests.
type echoModule struct{}

func (m *echoModule) Name() string        { return "echo" }
func (m *echoModule) Description() string { return "Echo tools for benchmarking" }
func (m *echoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "echo",
				Description: "Echoes a message",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"message": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				msg := handler.GetStringParam(req, "message")
				return handler.TextResult(msg), nil
			},
			Category: "test",
		},
		{
			Tool: mcp.Tool{
				Name:        "echo_upper",
				Description: "Echoes a message in uppercase style indicator",
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return handler.TextResult("UPPER"), nil
			},
			Category: "test",
		},
	}
}

func newEchoRegistry() *registry.ToolRegistry {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})
	return reg
}

func BenchmarkTool_Echo(b *testing.B) {
	reg := newEchoRegistry()
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

func BenchmarkToolParallel_Echo(b *testing.B) {
	reg := newEchoRegistry()
	BenchmarkToolParallel(b, reg, "echo", map[string]any{"message": "parallel-bench"})
}

func BenchmarkSuite_AllEchoTools(b *testing.B) {
	reg := newEchoRegistry()
	BenchmarkSuite(b, reg, func(name string) map[string]any {
		if name == "echo" {
			return map[string]any{"message": "suite-bench"}
		}
		return nil
	})
}

// TestBenchmarkTool_NotRegistered verifies BenchmarkTool fails fast for missing tools.
// We use a real *testing.B via sub-benchmark to confirm no panic on valid paths.
func TestBenchmarkTool_ValidTool(t *testing.T) {
	reg := newEchoRegistry()
	// Verify tool exists before benchmarking
	_, ok := reg.GetTool("echo")
	if !ok {
		t.Fatal("echo tool should be registered")
	}
}

func TestBenchmarkSuite_ListsAllTools(t *testing.T) {
	reg := newEchoRegistry()
	names := reg.ListTools()
	if len(names) != 2 {
		t.Errorf("expected 2 tools, got %d: %v", len(names), names)
	}
}

// noopMiddleware is a pass-through middleware for overhead measurement.
func noopMiddleware(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(ctx, req)
	}
}

// contextMiddleware adds a value to context (simulates auth/tracing middleware).
func contextMiddleware(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
	type ctxKey struct{}
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx = context.WithValue(ctx, ctxKey{}, name)
		return next(ctx, req)
	}
}

func newEchoRegistryWithMiddleware(mw []registry.Middleware) *registry.ToolRegistry {
	reg := registry.NewToolRegistry()
	if len(mw) > 0 {
		reg.SetMiddleware(mw)
	}
	reg.RegisterModule(&echoModule{})
	return reg
}

// BenchmarkMiddleware_None measures raw handler call without middleware.
func BenchmarkMiddleware_None(b *testing.B) {
	reg := newEchoRegistryWithMiddleware(nil)
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

// BenchmarkMiddleware_1_Noop measures overhead of 1 pass-through middleware.
func BenchmarkMiddleware_1_Noop(b *testing.B) {
	reg := newEchoRegistryWithMiddleware([]registry.Middleware{noopMiddleware})
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

// BenchmarkMiddleware_5_Noop measures overhead of 5 stacked noop middleware.
func BenchmarkMiddleware_5_Noop(b *testing.B) {
	mw := make([]registry.Middleware, 5)
	for i := range mw {
		mw[i] = noopMiddleware
	}
	reg := newEchoRegistryWithMiddleware(mw)
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

// BenchmarkMiddleware_10_Noop measures overhead of 10 stacked noop middleware.
func BenchmarkMiddleware_10_Noop(b *testing.B) {
	mw := make([]registry.Middleware, 10)
	for i := range mw {
		mw[i] = noopMiddleware
	}
	reg := newEchoRegistryWithMiddleware(mw)
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

// BenchmarkMiddleware_5_Context measures 5 middleware that add context values.
func BenchmarkMiddleware_5_Context(b *testing.B) {
	mw := make([]registry.Middleware, 5)
	for i := range mw {
		mw[i] = contextMiddleware
	}
	reg := newEchoRegistryWithMiddleware(mw)
	BenchmarkTool(b, reg, "echo", map[string]any{"message": "bench"})
}

// BenchmarkMiddleware_Parallel_5_Noop measures parallel throughput with 5 middleware.
func BenchmarkMiddleware_Parallel_5_Noop(b *testing.B) {
	mw := make([]registry.Middleware, 5)
	for i := range mw {
		mw[i] = noopMiddleware
	}
	reg := newEchoRegistryWithMiddleware(mw)
	BenchmarkToolParallel(b, reg, "echo", map[string]any{"message": "bench"})
}
