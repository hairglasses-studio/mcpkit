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

func setupTestServerWithAll(t *testing.T) (*Server, *Client) {
	t.Helper()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)

	// Register a test resource
	registry.AddResourceToServer(s.MCP,
		registry.Resource{Name: "test://greeting", Description: "A greeting resource"},
		func(_ context.Context, _ *registry.ReadResourceRequest) (*registry.ReadResourceResult, error) {
			return &registry.ReadResourceResult{
				Contents: []*registry.ResourceContents{
					{URI: "test://greeting", Text: "Hello, world!"},
				},
			}, nil
		},
	)

	// Register a test prompt without args
	registry.AddPromptToServer(s.MCP,
		registry.Prompt{Name: "greeting", Description: "A greeting prompt"},
		func(_ context.Context, _ *registry.GetPromptRequest) (*registry.GetPromptResult, error) {
			return &registry.GetPromptResult{
				Description: "Greeting",
				Messages: []*registry.PromptMessage{
					{Role: registry.RoleUser, Content: registry.MakeTextContent("Say hello")},
					{Role: registry.RoleAssistant, Content: registry.MakeTextContent("Hello!")},
				},
			}, nil
		},
	)

	// Register a test prompt with args
	registry.AddPromptToServer(s.MCP,
		registry.Prompt{
			Name:        "personalized",
			Description: "A personalized prompt",
			Arguments: []*registry.PromptArgument{
				{Name: "name", Description: "The person's name", Required: true},
			},
		},
		func(_ context.Context, req *registry.GetPromptRequest) (*registry.GetPromptResult, error) {
			greeting := fmt.Sprintf("Hello, %s!", req.Params.Arguments["name"])
			return &registry.GetPromptResult{
				Description: "Personalized",
				Messages: []*registry.PromptMessage{
					{Role: registry.RoleAssistant, Content: registry.MakeTextContent(greeting)},
				},
			}, nil
		},
	)

	c := NewClient(t, s)
	return s, c
}
