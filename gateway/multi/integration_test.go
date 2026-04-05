package multi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- integration test infrastructure ---

// integrationGateway wraps a Router with all three protocol adapters
// registered and an httptest.Server ready for HTTP requests.
type integrationGateway struct {
	Server   *httptest.Server
	Router   *Router
	Registry *registry.ToolRegistry
}

// newIntegrationGateway creates a gateway server with MCP, A2A, and OpenAI
// adapters. The caller registers tools on the registry before calling this.
func newIntegrationGateway(t *testing.T, reg *registry.ToolRegistry) *integrationGateway {
	t.Helper()

	router := NewRouter(reg)
	router.Register(NewMCPAdapter())
	router.Register(NewA2AAdapter())
	router.Register(NewOpenAIAdapter())

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &integrationGateway{
		Server:   srv,
		Router:   router,
		Registry: reg,
	}
}

// registryWithEchoTool builds a ToolRegistry with a simple echo tool.
func registryWithEchoTool(t *testing.T) *registry.ToolRegistry {
	t.Helper()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "integration",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "echo"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					args := req.Params.Arguments
					var message string
					if args != nil {
						if m, ok := args.(map[string]any); ok {
							if v, ok := m["message"]; ok {
								message = fmt.Sprint(v)
							}
						}
					}
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{Type: "text", Text: "echo: " + message},
						},
					}, nil
				},
			},
			{
				Tool: mcp.Tool{Name: "add"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					args := req.Params.Arguments
					var a, b float64
					if m, ok := args.(map[string]any); ok {
						if v, ok := m["a"]; ok {
							a, _ = v.(float64)
						}
						if v, ok := m["b"]; ok {
							b, _ = v.(float64)
						}
					}
					result := a + b
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{Type: "text", Text: fmt.Sprintf("%.0f", result)},
						},
					}, nil
				},
			},
		},
	})

	return reg
}

// --- helpers for building protocol-specific requests ---

// buildMCPRequest creates an MCP JSON-RPC tools/call request body.
func buildMCPRequest(id any, toolName string, args map[string]any) []byte {
	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"id":      id,
		"params":  params,
	}
	b, _ := json.Marshal(req)
	return b
}

// buildA2ARequest creates an A2A JSON-RPC a2a.sendMessage request with a
// DataPart carrying {"skill": toolName, "arguments": args}.
func buildA2ARequest(id any, toolName string, args map[string]any) []byte {
	dataPart := a2atypes.NewDataPart(map[string]any{
		"skill":     toolName,
		"arguments": args,
	})
	msg := a2atypes.NewMessage(a2atypes.MessageRoleUser, dataPart)

	sendReq := a2atypes.SendMessageRequest{
		Message: msg,
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "a2a.sendMessage",
		"id":      id,
		"params":  sendReq,
	}
	b, _ := json.Marshal(req)
	return b
}

// buildOpenAIRequest creates an OpenAI chat completion request with a tool call.
func buildOpenAIRequest(toolCallID, toolName string, args map[string]any) []byte {
	argsJSON, _ := json.Marshal(args)
	req := map[string]any{
		"model": "test-model",
		"messages": []map[string]any{
			{
				"role": "assistant",
				"tool_calls": []map[string]any{
					{
						"id":   toolCallID,
						"type": "function",
						"function": map[string]any{
							"name":      toolName,
							"arguments": string(argsJSON),
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(req)
	return b
}

// --- test cases ---

// TestGateway_MCP_ToolCall verifies that an MCP JSON-RPC tools/call request
// reaches the correct tool and produces a valid JSON-RPC response.
func TestGateway_MCP_ToolCall(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))

	body := buildMCPRequest(1, "echo", map[string]any{"message": "integration"})
	req, err := http.NewRequest(http.MethodPost, gw.Server.URL+"/mcp",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}

	// Parse the JSON-RPC response.
	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, respBody)
	}

	if rpcResp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", rpcResp.JSONRPC)
	}
	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %+v", rpcResp.Error)
	}
	if rpcResp.Result.IsError {
		t.Error("result.isError should be false")
	}
	if len(rpcResp.Result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if rpcResp.Result.Content[0].Text != "echo: integration" {
		t.Errorf("text = %q, want 'echo: integration'", rpcResp.Result.Content[0].Text)
	}
}

// TestGateway_A2A_SendMessage verifies that an A2A JSON-RPC a2a.sendMessage
// request reaches the correct tool and produces a valid task response.
func TestGateway_A2A_SendMessage(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))

	body := buildA2ARequest("req-a2a-1", "echo", map[string]any{"message": "hello-a2a"})
	req, err := http.NewRequest(http.MethodPost, gw.Server.URL+"/a2a",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}

	// Parse the JSON-RPC response containing an A2A Task.
	// The A2A Part type uses custom MarshalJSON that flattens content
	// (e.g., {"text": "..."} instead of {"content": {...}}), so we use
	// a generic map for flexible traversal plus raw body checks.
	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			ID     string `json:"id"`
			Status struct {
				State string `json:"state"`
			} `json:"status"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, respBody)
	}

	if rpcResp.Result.Status.State != string(a2atypes.TaskStateCompleted) {
		t.Errorf("task state = %q, want %q", rpcResp.Result.Status.State, a2atypes.TaskStateCompleted)
	}

	if rpcResp.Result.ID == "" {
		t.Error("expected non-empty task ID")
	}

	// Verify the raw response body contains the echo text in an artifact.
	// A2A Parts serialize with custom MarshalJSON, so checking the raw JSON
	// is more reliable than re-marshaling parsed structs.
	if !strings.Contains(string(respBody), "echo: hello-a2a") {
		t.Errorf("response does not contain expected text.\nbody: %s", respBody)
	}

	// Verify artifacts array exists.
	if !strings.Contains(string(respBody), `"artifacts"`) {
		t.Errorf("response missing artifacts.\nbody: %s", respBody)
	}
}

// TestGateway_OpenAI_FunctionCall verifies that an OpenAI function call request
// reaches the correct tool and produces a valid chat completion response.
func TestGateway_OpenAI_FunctionCall(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))

	body := buildOpenAIRequest("call_123", "echo", map[string]any{"message": "from-openai"})
	req, err := http.NewRequest(http.MethodPost, gw.Server.URL+"/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}

	// Parse the OpenAI chat completion response.
	var chatResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role       string          `json:"role"`
				Content    json.RawMessage `json:"content"`
				ToolCallID string          `json:"tool_call_id"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, respBody)
	}

	if chatResp.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", chatResp.Object)
	}

	if len(chatResp.Choices) == 0 {
		t.Fatal("expected at least one choice")
	}

	choice := chatResp.Choices[0]
	if choice.Message.Role != "tool" {
		t.Errorf("role = %q, want tool", choice.Message.Role)
	}

	// Content is a JSON-encoded string.
	var content string
	if err := json.Unmarshal(choice.Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v, raw: %s", err, choice.Message.Content)
	}
	if content != "echo: from-openai" {
		t.Errorf("content = %q, want 'echo: from-openai'", content)
	}

	if choice.Message.ToolCallID != "call_123" {
		t.Errorf("tool_call_id = %q, want call_123", choice.Message.ToolCallID)
	}
}

// TestGateway_ProtocolDetection verifies that the router selects the correct
// adapter for each type of request based on headers, path, and body.
func TestGateway_ProtocolDetection(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))
	args := map[string]any{"message": "detect"}

	tests := []struct {
		name         string
		path         string
		headers      map[string]string
		body         []byte
		wantStatus   int
		wantProtocol string // marker text in the response to verify routing
	}{
		{
			name:    "MCP via header",
			path:    "/",
			headers: map[string]string{"MCP-Protocol-Version": "2025-11-25", "Content-Type": "application/json"},
			body:    buildMCPRequest(10, "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "MCP via path prefix",
			path:    "/mcp",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildMCPRequest(11, "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "MCP via body method",
			path:    "/",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildMCPRequest(12, "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "A2A via path prefix",
			path:    "/a2a",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildA2ARequest("a2a-detect", "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "A2A via body method",
			path:    "/",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildA2ARequest("a2a-detect-2", "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "OpenAI via path prefix",
			path:    "/v1/chat/completions",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildOpenAIRequest("oai-detect", "echo", args),
			wantStatus: http.StatusOK,
		},
		{
			name:    "OpenAI via body structure",
			path:    "/custom",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    buildOpenAIRequest("oai-detect-2", "echo", args),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest(http.MethodPost, gw.Server.URL+tt.path,
				bytes.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d\nbody: %s", resp.StatusCode, tt.wantStatus, respBody)
			}

			// Verify the response contains our echo text (tool was invoked).
			if !strings.Contains(string(respBody), "echo: detect") {
				t.Errorf("response does not contain 'echo: detect'\nbody: %s", respBody)
			}
		})
	}
}

// TestGateway_UnknownProtocol verifies that requests with no detectable protocol
// receive a 400 response.
func TestGateway_UnknownProtocol(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))

	// Send a request with no protocol signals: no relevant headers, no
	// recognized path prefix, no matching body structure.
	body := `{"some": "random", "data": true}`
	req, err := http.NewRequest(http.MethodPost, gw.Server.URL+"/unknown/path",
		strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 400\nbody: %s", resp.StatusCode, respBody)
	}

	// Verify the response includes information about supported protocols.
	respBody, _ := io.ReadAll(resp.Body)
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if _, ok := errResp["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
	if _, ok := errResp["supported_protocols"]; !ok {
		t.Error("expected 'supported_protocols' field in response")
	}
}

// TestGateway_CrossProtocol_SameTool verifies that the same underlying tool
// called via MCP, A2A, and OpenAI all return equivalent results.
func TestGateway_CrossProtocol_SameTool(t *testing.T) {
	t.Parallel()

	gw := newIntegrationGateway(t, registryWithEchoTool(t))
	args := map[string]any{"a": float64(3), "b": float64(7)}

	// Call "add" via MCP.
	mcpBody := buildMCPRequest(100, "add", args)
	mcpReq, _ := http.NewRequest(http.MethodPost, gw.Server.URL+"/mcp",
		bytes.NewReader(mcpBody))
	mcpReq.Header.Set("Content-Type", "application/json")
	mcpReq.Header.Set("MCP-Protocol-Version", "2025-11-25")

	mcpResp, err := http.DefaultClient.Do(mcpReq)
	if err != nil {
		t.Fatal("MCP request failed:", err)
	}
	mcpRespBody, _ := io.ReadAll(mcpResp.Body)
	mcpResp.Body.Close()

	if mcpResp.StatusCode != http.StatusOK {
		t.Fatalf("MCP status = %d, body = %s", mcpResp.StatusCode, mcpRespBody)
	}

	// Call "add" via A2A.
	a2aBody := buildA2ARequest("cross-a2a", "add", args)
	a2aReq, _ := http.NewRequest(http.MethodPost, gw.Server.URL+"/a2a",
		bytes.NewReader(a2aBody))
	a2aReq.Header.Set("Content-Type", "application/json")

	a2aResp, err := http.DefaultClient.Do(a2aReq)
	if err != nil {
		t.Fatal("A2A request failed:", err)
	}
	a2aRespBody, _ := io.ReadAll(a2aResp.Body)
	a2aResp.Body.Close()

	if a2aResp.StatusCode != http.StatusOK {
		t.Fatalf("A2A status = %d, body = %s", a2aResp.StatusCode, a2aRespBody)
	}

	// Call "add" via OpenAI.
	oaiBody := buildOpenAIRequest("cross-oai", "add", args)
	oaiReq, _ := http.NewRequest(http.MethodPost, gw.Server.URL+"/v1/chat/completions",
		bytes.NewReader(oaiBody))
	oaiReq.Header.Set("Content-Type", "application/json")

	oaiResp, err := http.DefaultClient.Do(oaiReq)
	if err != nil {
		t.Fatal("OpenAI request failed:", err)
	}
	oaiRespBody, _ := io.ReadAll(oaiResp.Body)
	oaiResp.Body.Close()

	if oaiResp.StatusCode != http.StatusOK {
		t.Fatalf("OpenAI status = %d, body = %s", oaiResp.StatusCode, oaiRespBody)
	}

	// All three responses should contain the result "10".
	if !strings.Contains(string(mcpRespBody), "10") {
		t.Errorf("MCP response missing '10': %s", mcpRespBody)
	}
	if !strings.Contains(string(a2aRespBody), "10") {
		t.Errorf("A2A response missing '10': %s", a2aRespBody)
	}
	if !strings.Contains(string(oaiRespBody), "10") {
		t.Errorf("OpenAI response missing '10': %s", oaiRespBody)
	}
}

// TestGateway_ConcurrentMultiProtocol fires 10 concurrent requests across
// all three protocols and verifies they all succeed without races or corruption.
func TestGateway_ConcurrentMultiProtocol(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	var invocations atomic.Int64

	reg.RegisterModule(&testModule{
		name: "concurrent",
		tools: []registry.ToolDefinition{
			{
				Tool: mcp.Tool{Name: "count"},
				Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					n := invocations.Add(1)
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{Type: "text", Text: fmt.Sprintf("call-%d", n)},
						},
					}, nil
				},
			},
		},
	})

	gw := newIntegrationGateway(t, reg)
	args := map[string]any{}

	const total = 10
	var wg sync.WaitGroup
	errs := make(chan error, total)

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var body []byte
			var url string
			var headers map[string]string

			switch idx % 3 {
			case 0: // MCP
				body = buildMCPRequest(idx, "count", args)
				url = gw.Server.URL + "/mcp"
				headers = map[string]string{
					"Content-Type":         "application/json",
					"MCP-Protocol-Version": "2025-11-25",
				}
			case 1: // A2A
				body = buildA2ARequest(fmt.Sprintf("conc-%d", idx), "count", args)
				url = gw.Server.URL + "/a2a"
				headers = map[string]string{
					"Content-Type": "application/json",
				}
			case 2: // OpenAI
				body = buildOpenAIRequest(fmt.Sprintf("conc-%d", idx), "count", args)
				url = gw.Server.URL + "/v1/chat/completions"
				headers = map[string]string{
					"Content-Type": "application/json",
				}
			}

			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				errs <- fmt.Errorf("[%d] build request: %w", idx, err)
				return
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errs <- fmt.Errorf("[%d] do request: %w", idx, err)
				return
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("[%d] status=%d body=%s", idx, resp.StatusCode, respBody)
				return
			}

			// Verify response contains "call-" indicating the tool was invoked.
			if !strings.Contains(string(respBody), "call-") {
				errs <- fmt.Errorf("[%d] response missing call marker: %s", idx, respBody)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all 10 requests were handled.
	got := invocations.Load()
	if got != total {
		t.Errorf("invocations = %d, want %d", got, total)
	}
}
