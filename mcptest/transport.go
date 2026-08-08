//go:build !official_sdk

package mcptest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

func (tr *transport) callTool(ctx context.Context, t testing.TB, name string, args map[string]any) (*registry.CallToolResult, error) {
	t.Helper()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
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

func (tr *transport) readResource(ctx context.Context, t testing.TB, uri string) (*registry.ReadResourceResult, error) {
	t.Helper()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": uri,
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
		Result json.RawMessage `json:"result"`
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

	result, err := mcp.ParseReadResourceResult(&rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("parse resource result: %w", err)
	}

	return result, nil
}

// doListRequest sends a no-params JSON-RPC request for method (tools/list,
// resources/list, or prompts/list) and returns the raw "result" object for
// the caller to unmarshal into whatever shape it needs.
func (tr *transport) doListRequest(ctx context.Context, t testing.TB, method string) (json.RawMessage, error) {
	t.Helper()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
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
		Result json.RawMessage `json:"result"`
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

func (tr *transport) listToolNames(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.doListRequest(ctx, t, "tools/list")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal tools/list result: %w", err)
	}
	names := make([]string, 0, len(parsed.Tools))
	for _, tl := range parsed.Tools {
		names = append(names, tl.Name)
	}
	return names, nil
}

func (tr *transport) listResourceURIs(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.doListRequest(ctx, t, "resources/list")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Resources []struct {
			URI string `json:"uri"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal resources/list result: %w", err)
	}
	uris := make([]string, 0, len(parsed.Resources))
	for _, r := range parsed.Resources {
		uris = append(uris, r.URI)
	}
	return uris, nil
}

func (tr *transport) listPromptNames(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.doListRequest(ctx, t, "prompts/list")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Prompts []struct {
			Name string `json:"name"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal prompts/list result: %w", err)
	}
	names := make([]string, 0, len(parsed.Prompts))
	for _, p := range parsed.Prompts {
		names = append(names, p.Name)
	}
	return names, nil
}

func (tr *transport) getPrompt(ctx context.Context, t testing.TB, name string, args map[string]string) (*registry.GetPromptResult, error) {
	t.Helper()

	params := map[string]any{
		"name": name,
	}
	if args != nil {
		params["arguments"] = args
	}

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/get",
		"params":  params,
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
		Result json.RawMessage `json:"result"`
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

	result, err := mcp.ParseGetPromptResult(&rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("parse prompt result: %w", err)
	}

	return result, nil
}
