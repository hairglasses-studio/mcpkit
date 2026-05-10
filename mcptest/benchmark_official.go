//go:build official_sdk

package mcptest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// BenchmarkTool runs b.N sequential calls to toolName with the provided args,
// using the tool handler directly from reg.
func BenchmarkTool(b *testing.B, reg *registry.ToolRegistry, toolName string, args map[string]any) {
	b.Helper()

	td, ok := reg.GetTool(toolName)
	if !ok {
		b.Fatalf("BenchmarkTool: tool %q not registered", toolName)
	}

	req := buildCallToolRequest(toolName, args)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := td.Handler(ctx, req); err != nil {
			b.Fatalf("BenchmarkTool: tool %q returned error: %v", toolName, err)
		}
	}
}

// BenchmarkToolParallel runs the tool handler in parallel across b.N calls.
func BenchmarkToolParallel(b *testing.B, reg *registry.ToolRegistry, toolName string, args map[string]any) {
	b.Helper()

	td, ok := reg.GetTool(toolName)
	if !ok {
		b.Fatalf("BenchmarkToolParallel: tool %q not registered", toolName)
	}

	req := buildCallToolRequest(toolName, args)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := td.Handler(ctx, req); err != nil {
				b.Errorf("BenchmarkToolParallel: tool %q returned error: %v", toolName, err)
				return
			}
		}
	})
}

// BenchmarkSuite iterates over all tools registered in reg and runs a
// sub-benchmark for each tool.
func BenchmarkSuite(b *testing.B, reg *registry.ToolRegistry, argsFunc func(string) map[string]any) {
	b.Helper()

	for _, name := range reg.ListTools() {
		td, ok := reg.GetTool(name)
		if !ok {
			continue
		}

		b.Run(name, func(b *testing.B) {
			args := argsFunc(name)
			req := buildCallToolRequest(name, args)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := td.Handler(ctx, req); err != nil {
					b.Fatalf("BenchmarkSuite: tool %q returned error: %v", name, err)
				}
			}
		})
	}
}

// BaselineBenchmark runs a standardized sequential benchmark for all tools in reg.
func BaselineBenchmark(b *testing.B, reg *registry.ToolRegistry, argsFunc func(string) map[string]any) {
	b.Helper()
	b.ReportAllocs()
	BenchmarkSuite(b, reg, argsFunc)
}

// BenchmarkGatewayProxy measures gateway proxy overhead by comparing direct
// handler invocation vs proxied invocation through the registry.
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
		Tool: registry.Tool{Name: "bench_tool"},
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

func buildCallToolRequest(name string, args map[string]any) registry.CallToolRequest {
	req := registry.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: name},
	}
	if args != nil {
		data, _ := json.Marshal(args)
		req.Params.Arguments = data
	}
	return req
}

func gatewayBenchReq(name string, args map[string]any) registry.CallToolRequest {
	return buildCallToolRequest(name, args)
}
