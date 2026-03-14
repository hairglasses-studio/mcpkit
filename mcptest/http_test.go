//go:build !official_sdk

package mcptest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func newTestRegistry() *registry.ToolRegistry {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&httpTestModule{})
	return reg
}

type httpTestModule struct{}

func (m *httpTestModule) Name() string        { return "httptest" }
func (m *httpTestModule) Description() string { return "HTTP test tools" }
func (m *httpTestModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "echo",
				Description: "Echoes the input message",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"message": map[string]any{"type": "string"},
					},
					Required: []string{"message"},
				},
			},
			Handler: func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				msg := ""
				if req.Params.Arguments != nil {
					if args, ok := req.Params.Arguments.(map[string]any); ok {
						if m, ok := args["message"].(string); ok {
							msg = m
						}
					}
				}
				return registry.MakeTextResult(fmt.Sprintf("echo: %s", msg)), nil
			},
		},
		{
			Tool: mcp.Tool{
				Name:        "fail",
				Description: "Always returns an error",
			},
			Handler: func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return registry.MakeErrorResult("[TEST_ERROR] intentional failure"), nil
			},
		},
	}
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	return resp
}

func postSessionJSON(t *testing.T, url, sessionID string, body any) *http.Response {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(server.HeaderKeySessionID, sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	return resp
}

func readJSONResponse(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response %q: %v", string(body), err)
	}
	return result
}

func initializeSession(t *testing.T, endpoint string) string {
	t.Helper()
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	resp := postJSON(t, endpoint, initReq)
	sessionID := resp.Header.Get(server.HeaderKeySessionID)
	if sessionID == "" {
		t.Fatal("no session ID in initialize response")
	}

	result := readJSONResponse(t, resp)
	if result["error"] != nil {
		t.Fatalf("initialize error: %v", result["error"])
	}

	// Send initialized notification
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifResp := postSessionJSON(t, endpoint, sessionID, notif)
	notifResp.Body.Close()

	return sessionID
}

func TestHTTPServer_Initialize(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
}

func TestHTTPServer_ToolCall(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())

	// Call echo tool
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "echo",
			"arguments": map[string]any{"message": "hello HTTP"},
		},
	}
	resp := postSessionJSON(t, srv.Endpoint(), sessionID, callReq)

	// Response may be SSE or JSON depending on server state
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	// Extract result — may be direct JSON or SSE event
	var resultJSON []byte
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		// Parse SSE: find the last "data: " line
		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				resultJSON = []byte(strings.TrimPrefix(line, "data: "))
			}
		}
		if resultJSON == nil {
			t.Fatalf("no data line in SSE response: %s", string(body))
		}
	} else {
		resultJSON = body
	}

	var rpcResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resultJSON, &rpcResp); err != nil {
		t.Fatalf("unmarshal result %q: %v", string(resultJSON), err)
	}
	if rpcResp.Error != nil {
		t.Fatalf("RPC error: %s", rpcResp.Error.Message)
	}
	if len(rpcResp.Result.Content) == 0 {
		t.Fatal("empty content")
	}
	if rpcResp.Result.Content[0].Text != "echo: hello HTTP" {
		t.Errorf("text = %q, want 'echo: hello HTTP'", rpcResp.Result.Content[0].Text)
	}
}

func TestHTTPServer_ToolList(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())

	listReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	}
	resp := postSessionJSON(t, srv.Endpoint(), sessionID, listReq)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Parse (handle SSE or JSON)
	var resultJSON []byte
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "data: ") {
				resultJSON = []byte(strings.TrimPrefix(line, "data: "))
			}
		}
	} else {
		resultJSON = body
	}

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resultJSON, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	names := make(map[string]bool)
	for _, tool := range rpcResp.Result.Tools {
		names[tool.Name] = true
	}
	if !names["echo"] {
		t.Error("echo tool not in tools/list")
	}
	if !names["fail"] {
		t.Error("fail tool not in tools/list")
	}
}

func TestHTTPServer_ErrorTool(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "fail",
		},
	}
	resp := postSessionJSON(t, srv.Endpoint(), sessionID, callReq)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var resultJSON []byte
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "data: ") {
				resultJSON = []byte(strings.TrimPrefix(line, "data: "))
			}
		}
	} else {
		resultJSON = body
	}

	var rpcResp struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resultJSON, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !rpcResp.Result.IsError {
		t.Error("expected isError=true")
	}
	if len(rpcResp.Result.Content) == 0 || !strings.Contains(rpcResp.Result.Content[0].Text, "TEST_ERROR") {
		t.Errorf("expected error text with TEST_ERROR, got %v", rpcResp.Result.Content)
	}
}

func TestHTTPServer_MissingContentType(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	// POST without Content-Type header should be rejected
	req, _ := http.NewRequest(http.MethodPost, srv.Endpoint(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	resp.Body.Close()
	// Server should reject with 415 Unsupported Media Type or 400 Bad Request
	if resp.StatusCode == http.StatusOK {
		t.Error("expected error status for missing Content-Type, got 200")
	}
}

func TestHTTPServer_SessionDelete(t *testing.T) {
	reg := newTestRegistry()
	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())

	// DELETE session
	req, _ := http.NewRequest(http.MethodDelete, srv.Endpoint(), nil)
	req.Header.Set(server.HeaderKeySessionID, sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send DELETE: %v", err)
	}
	resp.Body.Close()

	// 204 or 200 expected for successful delete
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("DELETE status = %d, want 204 or 200", resp.StatusCode)
	}
}

func TestHTTPServer_MiddlewareApplied(t *testing.T) {
	recorder := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{recorder.Middleware()},
	})
	reg.RegisterModule(&httpTestModule{})

	srv := NewHTTPServer(t, reg)
	defer srv.Close()

	sessionID := initializeSession(t, srv.Endpoint())

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "echo",
			"arguments": map[string]any{"message": "middleware test"},
		},
	}
	resp := postSessionJSON(t, srv.Endpoint(), sessionID, callReq)
	io.ReadAll(resp.Body)
	resp.Body.Close()

	recorder.AssertCallCount(t, 1)
	recorder.AssertCalled(t, "echo")
}
