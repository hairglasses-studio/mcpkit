//go:build official_sdk

// server_compat_smoke_official_test.go is the go-sdk counterpart to
// server_compat_smoke_test.go: same NewMCPServerWithOptions ->
// NewStreamableHTTPHandler compat pair, same initialize -> tools/call round
// trip, driven against go-sdk's StreamableHTTPHandler instead of mcp-go's.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewMCPServerWithOptions_SmokeHTTP(t *testing.T) {
	s := NewMCPServerWithOptions("smoke-server", "0.0.0-test", ServerOptions{
		Instructions:         "smoke test server",
		ToolCapabilities:     true,
		ResourceCapabilities: true,
		ResourceSubscribe:    false,
		ResourceListChanged:  false,
		PromptCapabilities:   false,
		StrictInputSchemas:   true,
		Recovery:             true,
	})
	if s == nil {
		t.Fatal("NewMCPServerWithOptions returned nil")
	}

	AddToolToServer(s, mcp.Tool{
		Name:        "smoke_echo",
		Description: "Echoes the input message",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		},
	}, func(_ context.Context, req CallToolRequest) (*CallToolResult, error) {
		msg, _ := ExtractArguments(req)["message"].(string)
		return MakeTextResult("echo: " + msg), nil
	})

	handler := NewStreamableHTTPHandler(s, HTTPServerOptions{
		EndpointPath: "/mcp",
		Stateless:    true,
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	initResp := smokePostOfficial(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"smoke-client","version":"1.0.0"}}}`)
	if initResp["error"] != nil {
		t.Fatalf("initialize error: %v", initResp["error"])
	}

	callResp := smokePostOfficial(t, ts.URL, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"smoke_echo","arguments":{"message":"hi"}}}`)
	if callResp["error"] != nil {
		t.Fatalf("tools/call error: %v", callResp["error"])
	}
	result, ok := callResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in response: %v", callResp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in result: %v", result)
	}
	first, _ := content[0].(map[string]any)
	if first["text"] != "echo: hi" {
		t.Errorf("text = %v, want %q", first["text"], "echo: hi")
	}
}

func smokePostOfficial(t *testing.T, url, body string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var payload []byte
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(raw), "\n") {
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				payload = []byte(after)
			}
		}
		if payload == nil {
			t.Fatalf("no data line in SSE response: %s", string(raw))
		}
	} else {
		payload = raw
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal response %q (status %d): %v", string(payload), resp.StatusCode, err)
	}
	return result
}
