package multi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPAdapter_Protocol(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()
	if got := a.Protocol(); got != ProtocolMCP {
		t.Errorf("Protocol() = %q, want %q", got, ProtocolMCP)
	}
}

func TestMCPAdapter_Detect_MCPHeader(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"protocol version", "MCP-Protocol-Version", "2025-11-25"},
		{"session id", "Mcp-Session-Id", "sess-abc-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set(tt.header, tt.value)

			matches, conf := a.Detect(req, nil)
			if !matches {
				t.Error("expected match on MCP header")
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("confidence = %v, want definitive", conf)
			}
		})
	}
}

func TestMCPAdapter_Detect_PathPrefix(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	paths := []string{"/mcp", "/mcp/", "/mcp/ws"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, path, nil)

			matches, conf := a.Detect(req, nil)
			if !matches {
				t.Errorf("path %q: expected match", path)
			}
			if conf != ConfidenceHigh {
				t.Errorf("path %q: confidence = %v, want high", path, conf)
			}
		})
	}
}

func TestMCPAdapter_Detect_JSONRPCMethod(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	methods := []string{"initialize", "ping", "tools/list", "tools/call", "resources/list", "prompts/get"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			body := `{"jsonrpc":"2.0","method":"` + method + `","id":1}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

			matches, conf := a.Detect(req, []byte(body))
			if !matches {
				t.Errorf("method %q: expected match", method)
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("method %q: confidence = %v, want definitive", method, conf)
			}
		})
	}
}

func TestMCPAdapter_Detect_NonMCP(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	tests := []struct {
		name string
		req  func() *http.Request
		peek []byte
	}{
		{
			"no signals",
			func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/random", nil)
			},
			nil,
		},
		{
			"a2a method",
			func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"a2a.sendMessage","id":1}`))
			},
			[]byte(`{"jsonrpc":"2.0","method":"a2a.sendMessage","id":1}`),
		},
		{
			"unknown method",
			func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"custom.method","id":1}`))
			},
			[]byte(`{"jsonrpc":"2.0","method":"custom.method","id":1}`),
		},
		{
			"openai path",
			func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			matches, _ := a.Detect(tt.req(), tt.peek)
			if matches {
				t.Error("expected no match for non-MCP request")
			}
		})
	}
}

func TestMCPAdapter_Decode_ToolsCall(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":42,"params":{"name":"echo","arguments":{"message":"hello"}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.Protocol != ProtocolMCP {
		t.Errorf("Protocol = %q, want mcp", canonical.Protocol)
	}
	if canonical.ToolName != "echo" {
		t.Errorf("ToolName = %q, want echo", canonical.ToolName)
	}
	if canonical.Arguments["message"] != "hello" {
		t.Errorf("Arguments[message] = %v, want hello", canonical.Arguments["message"])
	}
	if canonical.RequestID != "42" {
		t.Errorf("RequestID = %q, want 42", canonical.RequestID)
	}
	if canonical.Metadata["jsonrpc.method"] != "tools/call" {
		t.Errorf("Metadata[jsonrpc.method] = %q, want tools/call", canonical.Metadata["jsonrpc.method"])
	}
}

func TestMCPAdapter_Decode_ToolsCall_StringID(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":"req-abc","params":{"name":"search","arguments":{"query":"test"}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.RequestID != "req-abc" {
		t.Errorf("RequestID = %q, want req-abc", canonical.RequestID)
	}
}

func TestMCPAdapter_Decode_ToolsCall_WithProgressToken(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"long_task","arguments":{},"_meta":{"progressToken":"tok-123"}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.ToolName != "long_task" {
		t.Errorf("ToolName = %q, want long_task", canonical.ToolName)
	}
	if canonical.Metadata["mcp.progressToken"] != `"tok-123"` {
		t.Errorf("Metadata[mcp.progressToken] = %q, want \"tok-123\"", canonical.Metadata["mcp.progressToken"])
	}
}

func TestMCPAdapter_Decode_ToolsCall_NoArguments(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"ping"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.ToolName != "ping" {
		t.Errorf("ToolName = %q, want ping", canonical.ToolName)
	}
	if canonical.Arguments != nil && len(canonical.Arguments) != 0 {
		t.Errorf("Arguments = %v, want nil or empty", canonical.Arguments)
	}
}

func TestMCPAdapter_Decode_ToolsCall_MissingName(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"arguments":{"x":1}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for missing tool name")
	}
	if !strings.Contains(err.Error(), "missing tool name") {
		t.Errorf("error = %q, expected to contain 'missing tool name'", err)
	}
}

func TestMCPAdapter_Decode_ToolsCall_MissingParams(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"tools/call","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for missing params")
	}
	if !strings.Contains(err.Error(), "missing params") {
		t.Errorf("error = %q, expected to contain 'missing params'", err)
	}
}

func TestMCPAdapter_Decode_LifecycleMethods(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	methods := []string{"initialize", "ping", "tools/list", "resources/list", "prompts/get"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			body := `{"jsonrpc":"2.0","method":"` + method + `","id":1}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

			canonical, err := a.Decode(req)
			if err != nil {
				t.Fatalf("Decode(%q) error: %v", method, err)
			}

			if canonical.ToolName != "" {
				t.Errorf("ToolName = %q, want empty for lifecycle method", canonical.ToolName)
			}
			if canonical.Metadata["jsonrpc.method"] != method {
				t.Errorf("Metadata[jsonrpc.method] = %q, want %q", canonical.Metadata["jsonrpc.method"], method)
			}
		})
	}
}

func TestMCPAdapter_Decode_Notification(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.ToolName != "" {
		t.Errorf("ToolName = %q, want empty for notification", canonical.ToolName)
	}
}

func TestMCPAdapter_Decode_InvalidJSON(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `not json at all`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMCPAdapter_Decode_WrongJSONRPCVersion(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"1.0","method":"tools/call","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for wrong JSON-RPC version")
	}
	if !strings.Contains(err.Error(), "JSON-RPC version") {
		t.Errorf("error = %q, expected JSON-RPC version mention", err)
	}
}

func TestMCPAdapter_Decode_UnsupportedMethod(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	body := `{"jsonrpc":"2.0","method":"custom/unknown","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for unsupported method")
	}
	if !strings.Contains(err.Error(), "unsupported MCP method") {
		t.Errorf("error = %q, expected 'unsupported MCP method'", err)
	}
}

func TestMCPAdapter_Encode_Success_TextContent(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "hello world"},
		},
		RequestID: "42",
	}

	body, ct, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var rpcResp mcpJSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", rpcResp.JSONRPC)
	}
	if string(rpcResp.ID) != "42" {
		t.Errorf("id = %s, want 42", string(rpcResp.ID))
	}
	if rpcResp.Error != nil {
		t.Error("expected no error in success response")
	}

	// Verify the result contains the expected content.
	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result mcpCallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Error("result.isError should be false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content[0].type = %q, want text", result.Content[0].Type)
	}
	if result.Content[0].Text != "hello world" {
		t.Errorf("content[0].text = %q, want 'hello world'", result.Content[0].Text)
	}
}

func TestMCPAdapter_Encode_Success_StringID(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success:   true,
		Content:   []ContentPart{{Type: ContentTypeText, Text: "ok"}},
		RequestID: "req-abc",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(rpcResp.ID) != `"req-abc"` {
		t.Errorf("id = %s, want \"req-abc\"", string(rpcResp.ID))
	}
}

func TestMCPAdapter_Encode_Success_EmptyID(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success:   true,
		Content:   []ContentPart{{Type: ContentTypeText, Text: "ok"}},
		RequestID: "",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(rpcResp.ID) != "null" {
		t.Errorf("id = %s, want null for empty RequestID", string(rpcResp.ID))
	}
}

func TestMCPAdapter_Encode_Success_MultipleContent(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "line 1"},
			{Type: ContentTypeText, Text: "line 2"},
			{Type: ContentTypeJSON, JSON: map[string]string{"key": "value"}},
		},
		RequestID: "1",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	json.Unmarshal(body, &rpcResp)
	resultBytes, _ := json.Marshal(rpcResp.Result)

	var result mcpCallToolResult
	json.Unmarshal(resultBytes, &result)

	if len(result.Content) != 3 {
		t.Fatalf("content length = %d, want 3", len(result.Content))
	}
	if result.Content[0].Text != "line 1" {
		t.Errorf("content[0].text = %q, want 'line 1'", result.Content[0].Text)
	}
	if result.Content[2].Type != "text" {
		t.Errorf("JSON content should be serialized as text, got type %q", result.Content[2].Type)
	}
}

func TestMCPAdapter_Encode_Error_InvalidParams(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrInvalidParams,
			Message: "missing required field: name",
		},
		RequestID: "5",
	}

	body, ct, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}

	var rpcResp mcpJSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rpcResp.Error == nil {
		t.Fatal("expected JSON-RPC error for ErrInvalidParams")
	}
	if rpcResp.Error.Code != jsonRPCInvalidParams {
		t.Errorf("error code = %d, want %d", rpcResp.Error.Code, jsonRPCInvalidParams)
	}
	if rpcResp.Error.Message != "missing required field: name" {
		t.Errorf("error message = %q", rpcResp.Error.Message)
	}
}

func TestMCPAdapter_Encode_Error_NotFound(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrNotFound,
			Message: `tool "nonexistent" not found`,
		},
		RequestID: "6",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	json.Unmarshal(body, &rpcResp)

	if rpcResp.Error == nil {
		t.Fatal("expected JSON-RPC error for ErrNotFound")
	}
	if rpcResp.Error.Code != jsonRPCMethodNotFound {
		t.Errorf("error code = %d, want %d", rpcResp.Error.Code, jsonRPCMethodNotFound)
	}
}

func TestMCPAdapter_Encode_Error_ToolLevel(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrInternal,
			Message: "handler crashed",
		},
		RequestID: "7",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	json.Unmarshal(body, &rpcResp)

	// Tool-level errors should use isError=true, not JSON-RPC error.
	if rpcResp.Error != nil {
		t.Error("tool-level errors should use result.isError, not JSON-RPC error")
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result mcpCallToolResult
	json.Unmarshal(resultBytes, &result)

	if !result.IsError {
		t.Error("expected isError=true for tool-level error")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}
	if result.Content[0].Text != "handler crashed" {
		t.Errorf("content[0].text = %q, want 'handler crashed'", result.Content[0].Text)
	}
}

func TestMCPAdapter_Encode_Error_NilErrorInfo(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success:   false,
		Error:     nil,
		RequestID: "8",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	json.Unmarshal(body, &rpcResp)

	if rpcResp.Error == nil {
		t.Fatal("expected JSON-RPC error for nil ErrorInfo")
	}
	if rpcResp.Error.Code != jsonRPCInternalError {
		t.Errorf("error code = %d, want %d", rpcResp.Error.Code, jsonRPCInternalError)
	}
}

func TestMCPAdapter_Encode_Success_EmptyContent(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	resp := &CanonicalResponse{
		Success:   true,
		Content:   nil,
		RequestID: "9",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp mcpJSONRPCResponse
	json.Unmarshal(body, &rpcResp)

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result mcpCallToolResult
	json.Unmarshal(resultBytes, &result)

	if result.Content == nil {
		t.Error("expected empty array, not null, for content")
	}
}

func TestMCPAdapter_Roundtrip(t *testing.T) {
	t.Parallel()

	a := NewMCPAdapter()

	// Simulate a full decode -> canonical -> encode cycle.
	body := `{"jsonrpc":"2.0","method":"tools/call","id":"req-rt","params":{"name":"search","arguments":{"query":"test","limit":10}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	// Simulate tool execution producing a response.
	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "found 3 results"},
		},
		RequestID: canonical.RequestID,
	}

	respBody, ct, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}

	// Verify the response is valid JSON-RPC.
	var rpcResp mcpJSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		t.Fatalf("invalid JSON-RPC response: %v", err)
	}

	if rpcResp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", rpcResp.JSONRPC)
	}
	if string(rpcResp.ID) != `"req-rt"` {
		t.Errorf("id = %s, want \"req-rt\"", string(rpcResp.ID))
	}
	if rpcResp.Error != nil {
		t.Error("unexpected error in response")
	}
}

func TestFormatJSONRPCID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"number", "42", "42"},
		{"string", `"req-abc"`, "req-abc"},
		{"null", "null", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatJSONRPCID(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("formatJSONRPCID(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMarshalRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want string
	}{
		{"number", "42", "42"},
		{"string", "req-abc", `"req-abc"`},
		{"empty", "", "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := marshalRequestID(tt.id)
			if string(got) != tt.want {
				t.Errorf("marshalRequestID(%q) = %s, want %s", tt.id, string(got), tt.want)
			}
		})
	}
}

func TestCanonicalToMCPContent(t *testing.T) {
	t.Parallel()

	t.Run("text content", func(t *testing.T) {
		t.Parallel()
		parts := []ContentPart{{Type: ContentTypeText, Text: "hello"}}
		result := canonicalToMCPContent(parts)
		if len(result) != 1 || result[0].Type != "text" || result[0].Text != "hello" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("json content", func(t *testing.T) {
		t.Parallel()
		parts := []ContentPart{{Type: ContentTypeJSON, JSON: map[string]int{"count": 5}}}
		result := canonicalToMCPContent(parts)
		if len(result) != 1 || result[0].Type != "text" {
			t.Errorf("JSON should serialize as text, got: %+v", result)
		}
		if !strings.Contains(result[0].Text, `"count"`) {
			t.Errorf("JSON text = %q, expected to contain count", result[0].Text)
		}
	})

	t.Run("image content", func(t *testing.T) {
		t.Parallel()
		parts := []ContentPart{{Type: ContentTypeImage, MimeType: "image/png", Data: []byte("base64data")}}
		result := canonicalToMCPContent(parts)
		if len(result) != 1 || result[0].Type != "image" {
			t.Errorf("unexpected result: %+v", result)
		}
		if result[0].MimeType != "image/png" {
			t.Errorf("mimeType = %q", result[0].MimeType)
		}
	})

	t.Run("empty parts", func(t *testing.T) {
		t.Parallel()
		result := canonicalToMCPContent(nil)
		if result == nil {
			t.Error("expected empty slice, not nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 parts, got %d", len(result))
		}
	})
}
