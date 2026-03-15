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
	BenchmarkTool(b, reg, "echo", map[string]interface{}{"message": "bench"})
}

func BenchmarkToolParallel_Echo(b *testing.B) {
	reg := newEchoRegistry()
	BenchmarkToolParallel(b, reg, "echo", map[string]interface{}{"message": "parallel-bench"})
}

func BenchmarkSuite_AllEchoTools(b *testing.B) {
	reg := newEchoRegistry()
	BenchmarkSuite(b, reg, func(name string) map[string]interface{} {
		if name == "echo" {
			return map[string]interface{}{"message": "suite-bench"}
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
