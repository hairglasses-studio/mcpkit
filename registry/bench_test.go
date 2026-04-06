package registry

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkGetTool measures single tool lookup by name (map read under RLock).
func BenchmarkGetTool(b *testing.B) {
	r := NewToolRegistry()
	// Register 100 tools across 10 modules to simulate a real server.
	for m := 0; m < 10; m++ {
		tools := make([]ToolDefinition, 10)
		for t := 0; t < 10; t++ {
			name := fmt.Sprintf("mod%d_tool%d", m, t)
			tools[t] = newTestTool(name, fmt.Sprintf("cat%d", m), nil)
		}
		r.RegisterModule(&testModule{name: fmt.Sprintf("mod%d", m), tools: tools})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.GetTool("mod5_tool7")
	}
}

// BenchmarkCallTool measures end-to-end tool invocation through the wrapped
// handler chain (timeout enforcement, panic recovery, truncation check).
func BenchmarkCallTool(b *testing.B) {
	handler := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name:  "bench",
		tools: []ToolDefinition{newTestTool("bench_tool", "bench", handler)},
	})

	// Get the wrapped handler (same path as RegisterWithServer).
	td, _ := r.GetTool("bench_tool")
	wrapped := r.wrapHandler("bench_tool", td)

	ctx := context.Background()
	req := CallToolRequest{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapped(ctx, req)
	}
}

// BenchmarkMiddlewareChain measures handler invocation with a 3-deep middleware
// chain, which is the typical production depth (audit + safety + signing).
func BenchmarkMiddlewareChain(b *testing.B) {
	noop := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}

	// 3-layer pass-through middleware.
	passthrough := func(_ string, _ ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			return next(ctx, req)
		}
	}

	r := NewToolRegistry(Config{
		Middleware: []Middleware{passthrough, passthrough, passthrough},
	})
	r.RegisterModule(&testModule{
		name:  "bench",
		tools: []ToolDefinition{newTestTool("bench_tool", "bench", noop)},
	})

	td, _ := r.GetTool("bench_tool")
	wrapped := r.wrapHandler("bench_tool", td)

	ctx := context.Background()
	req := CallToolRequest{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapped(ctx, req)
	}
}

// BenchmarkSearchTools measures the full-text search path with TF-IDF scoring
// and fuzzy matching across a populated registry.
func BenchmarkSearchTools(b *testing.B) {
	r := NewToolRegistry()
	for m := 0; m < 10; m++ {
		tools := make([]ToolDefinition, 10)
		for t := 0; t < 10; t++ {
			name := fmt.Sprintf("mod%d_tool%d", m, t)
			td := newTestTool(name, fmt.Sprintf("category%d", m), nil)
			td.Tags = []string{"config", "desktop", "management"}
			td.SearchTerms = []string{"dotfiles", "reload", "hyprland"}
			tools[t] = td
		}
		r.RegisterModule(&testModule{name: fmt.Sprintf("mod%d", m), tools: tools})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.SearchTools("hyprland config")
	}
}

// BenchmarkListTools measures sorted tool name listing (the tools/list path).
func BenchmarkListTools(b *testing.B) {
	r := NewToolRegistry()
	for m := 0; m < 10; m++ {
		tools := make([]ToolDefinition, 10)
		for t := 0; t < 10; t++ {
			name := fmt.Sprintf("mod%d_tool%d", m, t)
			tools[t] = newTestTool(name, fmt.Sprintf("cat%d", m), nil)
		}
		r.RegisterModule(&testModule{name: fmt.Sprintf("mod%d", m), tools: tools})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ListTools()
	}
}
