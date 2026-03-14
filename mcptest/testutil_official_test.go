//go:build official_sdk

package mcptest

import (
	"context"
	"fmt"
	"testing"

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
			Tool: registry.Tool{
				Name:        "test_echo",
				Description: "Echoes input",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				msg := handler.GetStringParam(req, "message")
				return handler.TextResult(msg), nil
			},
			Category: "test",
		},
		{
			Tool: registry.Tool{
				Name:        "test_error",
				Description: "Always errors",
				InputSchema: map[string]any{"type": "object"},
			},
			Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
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
