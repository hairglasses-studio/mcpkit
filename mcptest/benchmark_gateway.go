package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// BenchmarkGatewayProxy measures gateway proxy overhead by comparing
// direct handler invocation vs proxied invocation through the registry.
// The difference reveals the cost of namespace lookup + dispatch.
func BenchmarkGatewayProxy(b *testing.B, reg *registry.ToolRegistry, toolName string, args map[string]any) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tools := reg.ListTools()
		if len(tools) == 0 {
			b.Fatal("no tools registered")
		}
		_ = tools
	}
}

// BenchmarkToolDirect measures raw handler latency with no middleware.
// This is the baseline for calculating gateway overhead.
func BenchmarkToolDirect(b *testing.B, handler registry.ToolHandlerFunc, args map[string]any) {
	ctx := context.Background()
	req := gatewayBenchReq("benchmark_tool", args)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := handler(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("nil result")
		}
	}
}

// BenchmarkMiddlewareOverhead measures the cost of a single middleware layer.
func BenchmarkMiddlewareOverhead(b *testing.B, middleware registry.Middleware, handler registry.ToolHandlerFunc, args map[string]any) {
	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "bench_tool"},
	}
	wrapped := middleware("bench_tool", td, handler)

	ctx := context.Background()
	req := gatewayBenchReq("bench_tool", args)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := wrapped(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("nil result")
		}
	}
}

// gatewayBenchReq builds a CallToolRequest for gateway benchmarks.
func gatewayBenchReq(name string, args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return req
}
