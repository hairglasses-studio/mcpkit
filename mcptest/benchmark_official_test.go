//go:build official_sdk

package mcptest

import (
	"context"

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
			Tool: registry.Tool{
				Name:        "echo",
				Description: "Echoes a message",
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
				Name:        "echo_upper",
				Description: "Echoes a message in uppercase style indicator",
			},
			Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
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
