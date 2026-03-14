package mcptest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Client provides test helper methods for calling tools.
type Client struct {
	t       testing.TB
	srv     *Server
	session *server.InProcessSession
}

// NewClient creates a test client connected to the given test server.
func NewClient(t testing.TB, s *Server) *Client {
	t.Helper()

	session := server.NewInProcessSession(server.GenerateInProcessSessionID(), nil)
	session.Initialize()
	if err := s.MCP.RegisterSession(context.Background(), session); err != nil {
		t.Fatalf("failed to register session: %v", err)
	}

	return &Client{
		t:       t,
		srv:     s,
		session: session,
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
	return c.callThroughServer(context.Background(), name, args)
}

// CallToolWithContext calls a tool with a custom context.
func (c *Client) CallToolWithContext(ctx context.Context, name string, args map[string]interface{}) *mcp.CallToolResult {
	c.t.Helper()
	result, err := c.callThroughServer(ctx, name, args)
	if err != nil {
		c.t.Fatalf("CallToolWithContext(%s): %v", name, err)
	}
	return result
}

// callThroughServer sends a JSON-RPC tools/call request through the MCP server,
// ensuring all middleware (including the recorder) is invoked.
func (c *Client) callThroughServer(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	c.t.Helper()

	// Build JSON-RPC request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Set session in context
	ctx = c.srv.MCP.WithContext(ctx, c.session)

	// Send through server
	resp := c.srv.MCP.HandleMessage(ctx, reqBytes)
	if resp == nil {
		return nil, fmt.Errorf("nil response from server")
	}

	// Marshal and re-parse the response to extract the result
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	var rpcResp struct {
		Result *mcp.CallToolResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}
