//go:build official_sdk

// server_compat_official.go — MCP server integration (official go-sdk variant).
package registry

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer is the server type used for tool/resource/prompt registration.
type MCPServer = mcp.Server

// ResourceHandlerFunc is the server-level resource handler signature.
type ResourceHandlerFunc = func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)

// PromptHandlerFunc is the server-level prompt handler signature.
type PromptHandlerFunc = func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// NewMCPServer creates a new MCP server instance.
func NewMCPServer(name, version string, opts ...any) *MCPServer {
	return mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: version,
	}, nil)
}

// AddToolToServer registers a tool with the MCP server.
// Adapts from mcpkit's value-receiver handler to official SDK's pointer-receiver handler.
func AddToolToServer(s *MCPServer, tool mcp.Tool, handler ToolHandlerFunc) {
	s.AddTool(&tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handler(ctx, *req)
	})
}

// AddResourceToServer registers a resource with the MCP server.
func AddResourceToServer(s *MCPServer, resource mcp.Resource, handler ResourceHandlerFunc) {
	s.AddResource(&resource, mcp.ResourceHandler(handler))
}

// AddResourceTemplateToServer registers a resource template with the MCP server.
func AddResourceTemplateToServer(s *MCPServer, tmpl mcp.ResourceTemplate, handler ResourceHandlerFunc) {
	s.AddResourceTemplate(&tmpl, mcp.ResourceHandler(handler))
}

// AddPromptToServer registers a prompt with the MCP server.
func AddPromptToServer(s *MCPServer, prompt mcp.Prompt, handler PromptHandlerFunc) {
	s.AddPrompt(&prompt, mcp.PromptHandler(handler))
}

// ServeStdio starts the MCP server on stdin/stdout.
func ServeStdio(s *MCPServer) error {
	ctx := context.Background()
	_, err := s.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		return err
	}
	// Block until stdin closes
	<-ctx.Done()
	return ctx.Err()
}

// RemoveToolsFromServer removes tools from the MCP server by name.
func RemoveToolsFromServer(s *MCPServer, names ...string) {
	s.RemoveTools(names...)
}

// RemoveResourcesFromServer removes resources from the MCP server by URI.
func RemoveResourcesFromServer(s *MCPServer, uris ...string) {
	s.RemoveResources(uris...)
}

// RemovePromptsFromServer removes prompts from the MCP server by name.
func RemovePromptsFromServer(s *MCPServer, names ...string) {
	s.RemovePrompts(names...)
}
