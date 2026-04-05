package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/gateway/multi"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ----- Protocol detection benchmarks -----

// BenchmarkDetectProtocol_MCP measures MCP detection via the
// MCP-Protocol-Version header (cheapest path, no body read).
func BenchmarkDetectProtocol_MCP(b *testing.B) {
	b.ReportAllocs()

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"jsonrpc":"2.0","method":"tools/call","id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","id":1}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = multi.DetectProtocol(req, body)
	}
}

// BenchmarkDetectProtocol_A2A measures A2A detection via body peek
// (JSON-RPC method extraction path).
func BenchmarkDetectProtocol_A2A(b *testing.B) {
	b.ReportAllocs()

	req := httptest.NewRequest(http.MethodPost, "/a2a",
		strings.NewReader(`{"jsonrpc":"2.0","method":"a2a.sendMessage","id":1}`))
	req.Header.Set("Content-Type", "application/json")

	body := []byte(`{"jsonrpc":"2.0","method":"a2a.sendMessage","id":1}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = multi.DetectProtocol(req, body)
	}
}

// BenchmarkDetectProtocol_OpenAI measures OpenAI detection via body
// structure analysis (tool_calls field scan).
func BenchmarkDetectProtocol_OpenAI(b *testing.B) {
	b.ReportAllocs()

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4","tool_calls":[{"id":"call_1","function":{"name":"test"}}]}`))
	req.Header.Set("Content-Type", "application/json")

	body := []byte(`{"model":"gpt-4","tool_calls":[{"id":"call_1","function":{"name":"test"}}]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = multi.DetectProtocol(req, body)
	}
}

// ----- Gateway round-trip benchmarks -----

// echoHandler returns whatever "message" argument it receives as text.
func echoHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := registry.ExtractArguments(req)
	msg := fmt.Sprint(args["message"])
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: msg},
		},
	}, nil
}

// benchRouter builds a Router with all three adapters registered and a
// single "echo" tool in the registry. Uses a discard logger to avoid
// benchmark noise from slog output.
func benchRouter(b *testing.B) *multi.Router {
	b.Helper()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&benchRouterModule{})

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := multi.NewRouter(reg, multi.WithLogger(quiet))
	router.Register(multi.NewMCPAdapter())
	router.Register(multi.NewA2AAdapter())
	router.Register(multi.NewOpenAIAdapter())
	return router
}

// benchRouterModule provides an echo tool for gateway benchmarks.
type benchRouterModule struct{}

func (m *benchRouterModule) Name() string        { return "bench_router" }
func (m *benchRouterModule) Description() string { return "benchmark router module" }
func (m *benchRouterModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.NewTool("echo",
				mcp.WithDescription("Echo back the message"),
				mcp.WithString("message", mcp.Required(), mcp.Description("Message to echo")),
			),
			Handler: echoHandler,
		},
	}
}

// mcpRequestBody is a pre-serialized MCP tools/call JSON-RPC request.
var mcpRequestBody = `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"echo","arguments":{"message":"benchmark"}}}`

// a2aRequestBody is a pre-serialized A2A sendMessage JSON-RPC request.
// Uses the DataPart format expected by the A2A adapter's Decode method.
var a2aRequestBody = `{
	"jsonrpc": "2.0",
	"method": "a2a.sendMessage",
	"id": "bench-1",
	"params": {
		"message": {
			"messageId": "msg-bench-1",
			"role": "user",
			"parts": [
				{
					"data": {
						"skill": "echo",
						"arguments": {
							"message": "benchmark"
						}
					}
				}
			]
		}
	}
}`

// openaiRequestBody is a pre-serialized OpenAI chat completion request with
// an assistant message containing a tool_call, matching the format expected
// by the OpenAI adapter's Decode method.
var openaiRequestBody = func() string {
	body := map[string]any{
		"model": "mcpkit-gateway",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Run echo with message benchmark",
			},
			{
				"role": "assistant",
				"tool_calls": []map[string]any{
					{
						"id":   "call_bench_1",
						"type": "function",
						"function": map[string]any{
							"name":      "echo",
							"arguments": `{"message":"benchmark"}`,
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(body)
	return string(b)
}()

// BenchmarkRouter_MCP_RoundTrip measures a full MCP request through the
// gateway: detect -> decode -> invoke -> encode.
func BenchmarkRouter_MCP_RoundTrip(b *testing.B) {
	b.ReportAllocs()

	router := benchRouter(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mcp",
			strings.NewReader(mcpRequestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRouter_A2A_RoundTrip measures a full A2A request through the gateway.
func BenchmarkRouter_A2A_RoundTrip(b *testing.B) {
	b.ReportAllocs()

	router := benchRouter(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/a2a",
			strings.NewReader(a2aRequestBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRouter_OpenAI_RoundTrip measures a full OpenAI request through the gateway.
func BenchmarkRouter_OpenAI_RoundTrip(b *testing.B) {
	b.ReportAllocs()

	router := benchRouter(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions",
			strings.NewReader(openaiRequestBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRouter_Concurrent_Mixed measures concurrent throughput with a
// mixed workload of MCP, A2A, and OpenAI requests. This validates that the
// Router's sync.RWMutex does not become a bottleneck under parallel load.
func BenchmarkRouter_Concurrent_Mixed(b *testing.B) {
	b.ReportAllocs()

	router := benchRouter(b)

	// Pre-build request factories for each protocol.
	type requestFactory struct {
		path    string
		body    string
		headers map[string]string
	}

	factories := []requestFactory{
		{
			path: "/mcp",
			body: mcpRequestBody,
			headers: map[string]string{
				"Content-Type":         "application/json",
				"MCP-Protocol-Version": "2025-11-25",
			},
		},
		{
			path: "/a2a",
			body: a2aRequestBody,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			path: "/openai/v1/chat/completions",
			body: openaiRequestBody,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			f := factories[idx%len(factories)]
			idx++

			req := httptest.NewRequest(http.MethodPost, f.path,
				strings.NewReader(f.body))
			for k, v := range f.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}
