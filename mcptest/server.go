package mcptest

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Server wraps a ToolRegistry for testing.
type Server struct {
	t        testing.TB
	Registry *registry.ToolRegistry
	MCP      *registry.MCPServer
}

// Option configures a test Server.
type Option func(*Server)

// NewServer creates a test MCP server with the given registry.
// The server is ready to use immediately — no HTTP transport is needed.
func NewServer(t testing.TB, reg *registry.ToolRegistry, opts ...Option) *Server {
	t.Helper()

	s := &Server{
		t:        t,
		Registry: reg,
		MCP:      registry.NewMCPServer("mcptest", "0.0.0-test"),
	}

	for _, opt := range opts {
		opt(s)
	}

	reg.RegisterWithServer(s.MCP)

	return s
}

// HasTool returns true if the server has a tool with the given name.
func (s *Server) HasTool(name string) bool {
	_, ok := s.Registry.GetTool(name)
	return ok
}

// ToolNames returns the names of all registered tools.
func (s *Server) ToolNames() []string {
	return s.Registry.ListTools()
}
