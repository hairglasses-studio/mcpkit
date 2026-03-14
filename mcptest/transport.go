//go:build !official_sdk

package mcptest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// transport holds the SDK-specific session for communicating with the MCP server.
// When migrating to the official SDK, this file is replaced with a build-tagged
// alternative — the Client public API stays the same.
type transport struct {
	session *server.InProcessSession
	srv     *Server
}

func newTransport(t testing.TB, s *Server) transport {
	t.Helper()

	session := server.NewInProcessSession(server.GenerateInProcessSessionID(), nil)
	session.Initialize()
	if err := s.MCP.RegisterSession(context.Background(), session); err != nil {
		t.Fatalf("failed to register session: %v", err)
	}

	return transport{session: session, srv: s}
}

func (tr *transport) callTool(ctx context.Context, t testing.TB, name string, args map[string]interface{}) (*registry.CallToolResult, error) {
	t.Helper()

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

	ctx = tr.srv.MCP.WithContext(ctx, tr.session)

	resp := tr.srv.MCP.HandleMessage(ctx, reqBytes)
	if resp == nil {
		return nil, fmt.Errorf("nil response from server")
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	var rpcResp struct {
		Result *registry.CallToolResult `json:"result"`
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
