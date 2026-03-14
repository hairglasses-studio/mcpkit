//go:build !official_sdk

package mcptest

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// testModule is a simple module for testing.
type testModule struct{}

func (m *testModule) Name() string        { return "test" }
func (m *testModule) Description() string { return "Test module" }
func (m *testModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "test_echo",
				Description: "Echoes input",
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
				Name:        "test_error",
				Description: "Always errors",
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return handler.CodedErrorResult(handler.ErrNotFound, fmt.Errorf("item not found")), nil
			},
			Category: "test",
		},
	}
}

func setupTestServer(t *testing.T) (*Server, *Client) {
	t.Helper()
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)
	return s, c
}

func TestNewServer(t *testing.T) {
	s, _ := setupTestServer(t)
	if !s.HasTool("test_echo") {
		t.Error("server should have test_echo tool")
	}
	names := s.ToolNames()
	if len(names) != 2 {
		t.Errorf("tool count = %d, want 2", len(names))
	}
}

func TestClient_CallTool(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_echo", map[string]interface{}{"message": "hello"})
	AssertToolResult(t, result, "hello")
	AssertNotError(t, result)
}

func TestClient_CallToolError(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_error", nil)
	AssertError(t, result, "NOT_FOUND")
}

func TestAssertToolResultContains(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_echo", map[string]interface{}{"message": "hello world"})
	AssertToolResultContains(t, result, "world")
}

func TestRecorder(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "hi"})
	c.CallTool("test_echo", map[string]interface{}{"message": "bye"})

	rec.AssertCallCount(t, 2)
	rec.AssertCalled(t, "test_echo")

	calls := rec.CallsFor("test_echo")
	if len(calls) != 2 {
		t.Errorf("calls for test_echo = %d, want 2", len(calls))
	}
}

func TestRecorderReset(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "hi"})
	rec.Reset()
	rec.AssertCallCount(t, 0)
}
