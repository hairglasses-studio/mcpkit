//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// BenchmarkTool runs b.N sequential calls to toolName with the provided args,
// using the tool handler directly from reg (bypassing HTTP transport overhead).
func BenchmarkTool(b *testing.B, reg *registry.ToolRegistry, toolName string, args map[string]interface{}) {
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
func BenchmarkToolParallel(b *testing.B, reg *registry.ToolRegistry, toolName string, args map[string]interface{}) {
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

// BenchmarkSuite iterates over all tools registered in reg and runs a sub-benchmark
// for each tool. argsFunc is called with the tool name to obtain the args map;
// it may return nil if no arguments are needed.
func BenchmarkSuite(b *testing.B, reg *registry.ToolRegistry, argsFunc func(string) map[string]interface{}) {
	b.Helper()

	for _, name := range reg.ListTools() {
		name := name // capture loop variable
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

// buildCallToolRequest constructs a mcp.CallToolRequest for the given tool and args.
func buildCallToolRequest(name string, args map[string]interface{}) registry.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	if args != nil {
		req.Params.Arguments = args
	}
	return req
}
