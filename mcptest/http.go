//go:build !official_sdk

package mcptest

import (
	"net/http/httptest"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// HTTPServer wraps an httptest.Server for testing MCP over streamable HTTP.
type HTTPServer struct {
	*httptest.Server
	registry *registry.ToolRegistry
}

// NewHTTPServer creates a test MCP server accessible over HTTP.
// It uses mcp-go's streamable HTTP transport with stateful sessions.
func NewHTTPServer(t interface{ Helper(); Fatalf(string, ...any) }, reg *registry.ToolRegistry, opts ...server.StreamableHTTPOption) *HTTPServer {
	t.Helper()

	mcpServer := registry.NewMCPServer("mcptest-http", "0.0.0-test")
	reg.RegisterWithServer(mcpServer)

	defaultOpts := []server.StreamableHTTPOption{
		server.WithStateful(true),
	}
	allOpts := append(defaultOpts, opts...)

	ts := server.NewTestStreamableHTTPServer(mcpServer, allOpts...)

	return &HTTPServer{
		Server:   ts,
		registry: reg,
	}
}

// Endpoint returns the full URL for MCP requests (the server URL itself,
// since NewTestStreamableHTTPServer routes at root).
func (s *HTTPServer) Endpoint() string {
	return s.URL + "/mcp"
}
