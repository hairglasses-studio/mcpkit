package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Client provides test helper methods for calling tools.
type Client struct {
	t  testing.TB
	tr transport
}

// NewClient creates a test client connected to the given test server.
func NewClient(t testing.TB, s *Server) *Client {
	t.Helper()

	return &Client{
		t:  t,
		tr: newTransport(t, s),
	}
}

// CallTool calls a tool by name with the given arguments.
// It fails the test if the tool is not found or returns an error.
func (c *Client) CallTool(name string, args map[string]interface{}) *mcp.CallToolResult {
	c.t.Helper()
	result, err := c.CallToolE(name, args)
	if err != nil {
		c.t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

// CallToolE calls a tool by name, returning both result and error.
func (c *Client) CallToolE(name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	c.t.Helper()
	return c.tr.callTool(context.Background(), c.t, name, args)
}

// CallToolWithContext calls a tool with a custom context.
func (c *Client) CallToolWithContext(ctx context.Context, name string, args map[string]interface{}) *mcp.CallToolResult {
	c.t.Helper()
	result, err := c.tr.callTool(ctx, c.t, name, args)
	if err != nil {
		c.t.Fatalf("CallToolWithContext(%s): %v", name, err)
	}
	return result
}
