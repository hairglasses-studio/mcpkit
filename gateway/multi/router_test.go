package multi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// stubAdapter is a test adapter with configurable behavior.
type stubAdapter struct {
	protocol   Protocol
	detectConf Confidence
	detectOK   bool
	decodeReq  *CanonicalRequest
	decodeErr  error
	encodeResp []byte
	encodeCT   string
	encodeErr  error
}

func (s *stubAdapter) Protocol() Protocol { return s.protocol }

func (s *stubAdapter) Detect(r *http.Request, bodyPeek []byte) (bool, Confidence) {
	return s.detectOK, s.detectConf
}

func (s *stubAdapter) Decode(r *http.Request) (*CanonicalRequest, error) {
	if s.decodeErr != nil {
		return nil, s.decodeErr
	}
	return s.decodeReq, nil
}

func (s *stubAdapter) Encode(resp *CanonicalResponse) ([]byte, string, error) {
	if s.encodeErr != nil {
		return nil, "", s.encodeErr
	}
	if s.encodeResp != nil {
		return s.encodeResp, s.encodeCT, nil
	}
	// Default: JSON-encode the response.
	body, err := json.Marshal(resp)
	return body, "application/json", err
}

func TestRouter_Register(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)

	adapter := &stubAdapter{protocol: ProtocolMCP}
	router.Register(adapter)

	protocols := router.Adapters()
	if len(protocols) != 1 {
		t.Fatalf("Adapters() = %d, want 1", len(protocols))
	}
	if protocols[0] != ProtocolMCP {
		t.Errorf("Adapters()[0] = %q, want mcp", protocols[0])
	}
}

func TestRouter_RegisterMultiple(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)

	router.Register(&stubAdapter{protocol: ProtocolMCP})
	router.Register(&stubAdapter{protocol: ProtocolA2A})
	router.Register(&stubAdapter{protocol: ProtocolOpenAI})

	protocols := router.Adapters()
	if len(protocols) != 3 {
		t.Fatalf("Adapters() = %d, want 3", len(protocols))
	}
}

func TestRouter_RegisterReplaces(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)

	router.Register(&stubAdapter{protocol: ProtocolMCP, detectOK: false})
	router.Register(&stubAdapter{protocol: ProtocolMCP, detectOK: true})

	protocols := router.Adapters()
	if len(protocols) != 1 {
		t.Fatalf("Adapters() = %d, want 1 after replace", len(protocols))
	}
}

func TestRouter_ServeHTTP_Success(t *testing.T) {
	t.Parallel()

	// Set up a registry with a test tool.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "echo"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					args := registry.ExtractArguments(req)
					msg := fmt.Sprint(args["message"])
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{Type: "text", Text: "echo: " + msg},
						},
					}, nil
				},
			},
		},
	})

	router := NewRouter(reg)
	router.Register(&stubAdapter{
		protocol:   ProtocolMCP,
		detectOK:   true,
		detectConf: ConfidenceDefinitive,
		decodeReq: &CanonicalRequest{
			Protocol:  ProtocolMCP,
			ToolName:  "echo",
			Arguments: map[string]any{"message": "hello"},
			RequestID: "req-1",
		},
	})

	// Send a request with MCP header so detection picks it up.
	body := `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"echo","arguments":{"message":"hello"}}}`
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var canonical CanonicalResponse
	if err := json.Unmarshal(respBody, &canonical); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !canonical.Success {
		t.Error("expected success response")
	}
	if len(canonical.Content) == 0 {
		t.Fatal("expected content")
	}
	if canonical.Content[0].Text != "echo: hello" {
		t.Errorf("content = %q, want 'echo: hello'", canonical.Content[0].Text)
	}
}

func TestRouter_ServeHTTP_ToolNotFound(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)
	router.Register(&stubAdapter{
		protocol:   ProtocolMCP,
		detectOK:   true,
		detectConf: ConfidenceDefinitive,
		decodeReq: &CanonicalRequest{
			Protocol:  ProtocolMCP,
			ToolName:  "nonexistent",
			RequestID: "req-2",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 for missing tool")
	}

	respBody, _ := io.ReadAll(resp.Body)
	var canonical CanonicalResponse
	if err := json.Unmarshal(respBody, &canonical); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if canonical.Success {
		t.Error("expected failure for missing tool")
	}
	if canonical.Error == nil || canonical.Error.Code != ErrNotFound {
		t.Error("expected ErrNotFound error code")
	}
}

func TestRouter_ServeHTTP_NoAdapter(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)
	// No adapters registered.

	req := httptest.NewRequest(http.MethodGet, "/random", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := errResp["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestRouter_ServeHTTP_DecodeError(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	router := NewRouter(reg)
	router.Register(&stubAdapter{
		protocol:   ProtocolMCP,
		detectOK:   true,
		detectConf: ConfidenceDefinitive,
		decodeErr:  fmt.Errorf("malformed request"),
	})

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRouter_ServeHTTP_HandlerError(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "err",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "fail"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					return nil, fmt.Errorf("handler exploded")
				},
			},
		},
	})

	router := NewRouter(reg)
	router.Register(&stubAdapter{
		protocol:   ProtocolMCP,
		detectOK:   true,
		detectConf: ConfidenceDefinitive,
		decodeReq: &CanonicalRequest{
			Protocol:  ProtocolMCP,
			ToolName:  "fail",
			RequestID: "req-err",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp := w.Result()
	respBody, _ := io.ReadAll(resp.Body)
	var canonical CanonicalResponse
	if err := json.Unmarshal(respBody, &canonical); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if canonical.Success {
		t.Error("expected failure when handler returns error")
	}
}

func TestRouter_ServeHTTP_IsErrorResult(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "errresult",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "bad_input"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					return registry.MakeErrorResult("invalid parameter: name"), nil
				},
			},
		},
	})

	router := NewRouter(reg)
	router.Register(&stubAdapter{
		protocol:   ProtocolMCP,
		detectOK:   true,
		detectConf: ConfidenceDefinitive,
		decodeReq: &CanonicalRequest{
			Protocol:  ProtocolMCP,
			ToolName:  "bad_input",
			RequestID: "req-bad",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	respBody, _ := io.ReadAll(w.Result().Body)
	var canonical CanonicalResponse
	if err := json.Unmarshal(respBody, &canonical); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if canonical.Success {
		t.Error("expected failure for IsError result")
	}
	if canonical.Error == nil {
		t.Fatal("expected error info")
	}
	if !strings.Contains(canonical.Error.Message, "invalid parameter") {
		t.Errorf("error message = %q, expected to contain 'invalid parameter'", canonical.Error.Message)
	}
}

func TestRouter_AdapterFallback(t *testing.T) {
	t.Parallel()

	// Verify that when global detection returns unknown, the router
	// still consults individual adapters.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "fb",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "ping"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					return &mcp.CallToolResult{
						Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "pong"}},
					}, nil
				},
			},
		},
	})

	router := NewRouter(reg)
	// Register an adapter that claims medium confidence via its own Detect.
	router.Register(&stubAdapter{
		protocol:   ProtocolOpenAI,
		detectOK:   true,
		detectConf: ConfidenceMedium,
		decodeReq: &CanonicalRequest{
			Protocol: ProtocolOpenAI,
			ToolName: "ping",
		},
	})

	// Send a request to a path that doesn't match any protocol.
	req := httptest.NewRequest(http.MethodPost, "/custom/endpoint",
		strings.NewReader(`{"tool_calls":[]}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 via adapter fallback", w.Result().StatusCode)
	}
}

func TestPeekRequestBody(t *testing.T) {
	t.Parallel()

	original := "hello world, this is a test body"
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(original))

	peek, newReq, err := peekRequestBody(req, 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(peek) != "hello" {
		t.Errorf("peek = %q, want 'hello'", string(peek))
	}

	// The full body should still be readable.
	full, err := io.ReadAll(newReq.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(full) != original {
		t.Errorf("full body = %q, want %q", string(full), original)
	}
}

func TestPeekRequestBody_NilBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	req.ContentLength = 0

	peek, _, err := peekRequestBody(req, 512)
	if err != nil {
		t.Fatal(err)
	}
	if peek != nil {
		t.Errorf("peek = %v, want nil for empty body", peek)
	}
}

func TestPeekRequestBody_SmallBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("ab"))

	peek, newReq, err := peekRequestBody(req, 512)
	if err != nil {
		t.Fatal(err)
	}
	if string(peek) != "ab" {
		t.Errorf("peek = %q, want 'ab'", string(peek))
	}

	full, _ := io.ReadAll(newReq.Body)
	if string(full) != "ab" {
		t.Errorf("full = %q, want 'ab'", string(full))
	}
}

// testModule implements registry.ToolModule for testing.
type testModule struct {
	name  string
	tools []registry.ToolDefinition
}

func (m *testModule) Name() string              { return m.name }
func (m *testModule) Description() string        { return "test module" }
func (m *testModule) Tools() []registry.ToolDefinition { return m.tools }
