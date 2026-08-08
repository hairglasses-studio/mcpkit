package mcptest

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
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
func (c *Client) CallTool(name string, args map[string]any) *registry.CallToolResult {
	c.t.Helper()
	result, err := c.CallToolE(name, args)
	if err != nil {
		c.t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

// CallToolE calls a tool by name, returning both result and error.
func (c *Client) CallToolE(name string, args map[string]any) (*registry.CallToolResult, error) {
	c.t.Helper()
	return c.tr.callTool(context.Background(), c.t, name, args)
}

// CallToolWithContext calls a tool with a custom context.
func (c *Client) CallToolWithContext(ctx context.Context, name string, args map[string]any) *registry.CallToolResult {
	c.t.Helper()
	result, err := c.tr.callTool(ctx, c.t, name, args)
	if err != nil {
		c.t.Fatalf("CallToolWithContext(%s): %v", name, err)
	}
	return result
}

// ListToolNames returns the names of every tool the server currently
// advertises via tools/list. It fails the test on transport error.
//
// This (plus ListResourceURIs/ListPromptNames) exists because Client has no
// full ListTools/ListResources/ListPrompts equivalent returning complete
// schema objects — building one portably would require exposing each SDK's
// list-result type through the compat layer, which no consumer has needed
// yet. Callers that only need to assert on a name set (the common case —
// "is tool X registered", "how many resources are there") should use these
// instead of reaching for an SDK-specific client directly.
func (c *Client) ListToolNames() []string {
	c.t.Helper()
	names, err := c.tr.listToolNames(context.Background(), c.t)
	if err != nil {
		c.t.Fatalf("ListToolNames: %v", err)
	}
	return names
}

// ListResourceURIs returns the URIs of every resource the server currently
// advertises via resources/list. It fails the test on transport error.
func (c *Client) ListResourceURIs() []string {
	c.t.Helper()
	uris, err := c.tr.listResourceURIs(context.Background(), c.t)
	if err != nil {
		c.t.Fatalf("ListResourceURIs: %v", err)
	}
	return uris
}

// ListPromptNames returns the names of every prompt the server currently
// advertises via prompts/list. It fails the test on transport error.
func (c *Client) ListPromptNames() []string {
	c.t.Helper()
	names, err := c.tr.listPromptNames(context.Background(), c.t)
	if err != nil {
		c.t.Fatalf("ListPromptNames: %v", err)
	}
	return names
}

// ReadResource reads a resource by URI.
// It fails the test if the resource is not found or returns an error.
func (c *Client) ReadResource(uri string) *registry.ReadResourceResult {
	c.t.Helper()
	result, err := c.ReadResourceE(uri)
	if err != nil {
		c.t.Fatalf("ReadResource(%s): %v", uri, err)
	}
	return result
}

// ReadResourceE reads a resource by URI, returning both result and error.
func (c *Client) ReadResourceE(uri string) (*registry.ReadResourceResult, error) {
	c.t.Helper()
	return c.tr.readResource(context.Background(), c.t, uri)
}

// GetPrompt gets a prompt by name with optional arguments.
// It fails the test if the prompt is not found or returns an error.
func (c *Client) GetPrompt(name string, args map[string]string) *registry.GetPromptResult {
	c.t.Helper()
	result, err := c.GetPromptE(name, args)
	if err != nil {
		c.t.Fatalf("GetPrompt(%s): %v", name, err)
	}
	return result
}

// GetPromptE gets a prompt by name, returning both result and error.
func (c *Client) GetPromptE(name string, args map[string]string) (*registry.GetPromptResult, error) {
	c.t.Helper()
	return c.tr.getPrompt(context.Background(), c.t, name, args)
}
