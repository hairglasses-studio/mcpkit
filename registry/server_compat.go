//go:build !official_sdk

// server_compat.go abstracts the MCP server integration layer (mcp-go variant).
//
// This file provides wrapper functions so that internal packages never
// import the server package directly. When migrating to a different SDK,
// only this file and compat.go need to change.
package registry

import (
	"context"
	"os"

	"github.com/hairglasses-studio/mcpkit/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer is the server type used for tool/resource/prompt registration.
type MCPServer = server.MCPServer

// ResourceHandlerFunc is the server-level resource handler signature.
type ResourceHandlerFunc = func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

// PromptHandlerFunc is the server-level prompt handler signature.
type PromptHandlerFunc = func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// NewMCPServer creates a new MCP server instance.
func NewMCPServer(name, version string, opts ...server.ServerOption) *MCPServer {
	return server.NewMCPServer(name, version, opts...)
}

// AddToolToServer registers a tool with the MCP server.
func AddToolToServer(s *MCPServer, tool mcp.Tool, handler ToolHandlerFunc) {
	s.AddTool(tool, server.ToolHandlerFunc(handler))
}

// AddResourceToServer registers a resource with the MCP server.
func AddResourceToServer(s *MCPServer, resource mcp.Resource, handler ResourceHandlerFunc) {
	s.AddResource(resource, server.ResourceHandlerFunc(handler))
}

// AddResourceTemplateToServer registers a resource template with the MCP server.
func AddResourceTemplateToServer(s *MCPServer, tmpl mcp.ResourceTemplate, handler ResourceHandlerFunc) {
	s.AddResourceTemplate(tmpl, server.ResourceTemplateHandlerFunc(handler))
}

// AddPromptToServer registers a prompt with the MCP server.
func AddPromptToServer(s *MCPServer, prompt mcp.Prompt, handler PromptHandlerFunc) {
	s.AddPrompt(prompt, server.PromptHandlerFunc(handler))
}

// ServeStdio starts the MCP server on stdin/stdout.
func ServeStdio(s *MCPServer) error {
	return server.ServeStdio(s)
}

// ServeAuto starts the MCP server via Unix socket if MCP_SOCKET_PATH is set,
// otherwise falls back to stdio. This allows existing servers to opt-in to
// socket pooling by setting a single environment variable.
func ServeAuto(s *MCPServer) error {
	if socketPath := os.Getenv("MCP_SOCKET_PATH"); socketPath != "" {
		srv := transport.NewUnixSocketServer(s, socketPath)
		return srv.Serve(context.Background())
	}
	return ServeStdio(s)
}

// RemoveToolsFromServer removes tools from the MCP server by name.
func RemoveToolsFromServer(s *MCPServer, names ...string) {
	s.DeleteTools(names...)
}

// RemoveResourcesFromServer removes resources from the MCP server by URI.
func RemoveResourcesFromServer(s *MCPServer, uris ...string) {
	s.DeleteResources(uris...)
}

// RemovePromptsFromServer removes prompts from the MCP server by name.
func RemovePromptsFromServer(s *MCPServer, names ...string) {
	s.DeletePrompts(names...)
}
