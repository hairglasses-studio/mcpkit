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

func setupTestServerWithAll(t *testing.T) (*Server, *Client) {
	t.Helper()
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)

	// Register a test resource
	registry.AddResourceToServer(s.MCP,
		mcp.NewResource("test://greeting", "greeting", mcp.WithResourceDescription("A greeting resource"), mcp.WithMIMEType("text/plain")),
		func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{URI: "test://greeting", Text: "Hello, world!"},
			}, nil
		},
	)

	// Register a test prompt without args
	registry.AddPromptToServer(s.MCP,
		mcp.NewPrompt("greeting", mcp.WithPromptDescription("A greeting prompt")),
		func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return mcp.NewGetPromptResult("Greeting", []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Say hello")),
				mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent("Hello!")),
			}), nil
		},
	)

	// Register a test prompt with args
	registry.AddPromptToServer(s.MCP,
		mcp.Prompt{
			Name:        "personalized",
			Description: "A personalized prompt",
			Arguments: []mcp.PromptArgument{
				{Name: "name", Description: "The person's name", Required: true},
			},
		},
		func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			name := req.Params.Name
			args := req.Params.Arguments
			greeting := fmt.Sprintf("Hello, %s!", args["name"])
			_ = name // used for routing
			return mcp.NewGetPromptResult("Personalized", []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(greeting)),
			}), nil
		},
	)

	c := NewClient(t, s)
	return s, c
}
