package multi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIAdapter_Protocol(t *testing.T) {
	t.Parallel()
	a := NewOpenAIAdapter()
	if a.Protocol() != ProtocolOpenAI {
		t.Errorf("Protocol() = %q, want openai", a.Protocol())
	}
}

func TestOpenAIAdapter_Detect_PathMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantOK   bool
		wantConf Confidence
	}{
		{"v1 chat completions", "/v1/chat/completions", true, ConfidenceHigh},
		{"openai prefixed", "/openai/v1/chat/completions", true, ConfidenceHigh},
		{"openai subpath", "/openai/v1/models", true, ConfidenceHigh},
		{"v1 chat subpath", "/v1/chat/stream", true, ConfidenceHigh},
		{"unrelated path", "/api/tools", false, ConfidenceLow},
		{"mcp path", "/mcp", false, ConfidenceLow},
		{"root", "/", false, ConfidenceLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := NewOpenAIAdapter()
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)

			ok, conf := a.Detect(req, nil)
			if ok != tt.wantOK {
				t.Errorf("Detect(path=%q) ok = %v, want %v", tt.path, ok, tt.wantOK)
			}
			if ok && conf != tt.wantConf {
				t.Errorf("Detect(path=%q) confidence = %v, want %v", tt.path, conf, tt.wantConf)
			}
		})
	}
}

func TestOpenAIAdapter_Detect_BodyStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		wantOK bool
	}{
		{
			"tool_calls in body",
			`{"model":"gpt-4","messages":[{"role":"assistant","tool_calls":[]}]}`,
			true,
		},
		{
			"function_call in body",
			`{"model":"gpt-4","function_call":{"name":"x"}}`,
			true,
		},
		{
			"functions array in body",
			`{"model":"gpt-4","functions":[{"name":"x"}]}`,
			true,
		},
		{
			"no openai structure",
			`{"jsonrpc":"2.0","method":"tools/call"}`,
			false,
		},
		{
			"empty body",
			``,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := NewOpenAIAdapter()
			req := httptest.NewRequest(http.MethodPost, "/custom", nil)

			ok, _ := a.Detect(req, []byte(tt.body))
			if ok != tt.wantOK {
				t.Errorf("Detect(body=%q) ok = %v, want %v", tt.body, ok, tt.wantOK)
			}
		})
	}
}

func TestOpenAIAdapter_Detect_RejectNonOpenAI(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()

	// MCP request — should not match.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","id":1}`)

	ok, _ := a.Detect(req, body)
	if ok {
		t.Error("Detect should reject MCP JSON-RPC request")
	}

	// A2A request — should not match.
	body = []byte(`{"jsonrpc":"2.0","method":"a2a.sendMessage","id":1}`)
	ok, _ = a.Detect(req, body)
	if ok {
		t.Error("Detect should reject A2A JSON-RPC request")
	}

	// GET request with no body — should not match.
	req = httptest.NewRequest(http.MethodGet, "/random", nil)
	ok, _ = a.Detect(req, nil)
	if ok {
		t.Error("Detect should reject GET with no openai signals")
	}
}

func TestOpenAIAdapter_Decode_SingleToolCall(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "What's the weather in SF?"},
			{
				"role": "assistant",
				"content": null,
				"tool_calls": [
					{
						"id": "call_abc123",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"city\":\"San Francisco\",\"units\":\"celsius\"}"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test-key")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if canonical.Protocol != ProtocolOpenAI {
		t.Errorf("Protocol = %q, want openai", canonical.Protocol)
	}
	if canonical.ToolName != "get_weather" {
		t.Errorf("ToolName = %q, want get_weather", canonical.ToolName)
	}
	if canonical.RequestID != "call_abc123" {
		t.Errorf("RequestID = %q, want call_abc123", canonical.RequestID)
	}
	if canonical.Arguments["city"] != "San Francisco" {
		t.Errorf("Arguments[city] = %v, want San Francisco", canonical.Arguments["city"])
	}
	if canonical.Arguments["units"] != "celsius" {
		t.Errorf("Arguments[units] = %v, want celsius", canonical.Arguments["units"])
	}
	if canonical.Auth == nil || canonical.Auth.Token != "sk-test-key" {
		t.Error("expected auth token from Authorization header")
	}
	if canonical.Metadata["openai_model"] != "gpt-4o" {
		t.Errorf("Metadata[openai_model] = %q, want gpt-4o", canonical.Metadata["openai_model"])
	}
}

func TestOpenAIAdapter_Decode_MultipleToolCalls(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_first",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"city\":\"SF\"}"
						}
					},
					{
						"id": "call_second",
						"type": "function",
						"function": {
							"name": "get_time",
							"arguments": "{\"timezone\":\"PST\"}"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// First tool call should be decoded.
	if canonical.ToolName != "get_weather" {
		t.Errorf("ToolName = %q, want get_weather (first tool call)", canonical.ToolName)
	}
	if canonical.RequestID != "call_first" {
		t.Errorf("RequestID = %q, want call_first", canonical.RequestID)
	}
}

func TestOpenAIAdapter_Decode_LegacyFunctionCall(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-3.5-turbo",
		"messages": [
			{
				"role": "assistant",
				"function_call": {
					"name": "search",
					"arguments": "{\"query\":\"golang mcp\"}"
				}
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if canonical.ToolName != "search" {
		t.Errorf("ToolName = %q, want search", canonical.ToolName)
	}
	if canonical.Arguments["query"] != "golang mcp" {
		t.Errorf("Arguments[query] = %v, want 'golang mcp'", canonical.Arguments["query"])
	}
	if canonical.RequestID != "legacy_fc" {
		t.Errorf("RequestID = %q, want legacy_fc for function_call format", canonical.RequestID)
	}
}

func TestOpenAIAdapter_Decode_EmptyArguments(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_empty",
						"type": "function",
						"function": {
							"name": "list_tools",
							"arguments": "{}"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if canonical.ToolName != "list_tools" {
		t.Errorf("ToolName = %q, want list_tools", canonical.ToolName)
	}
	if len(canonical.Arguments) != 0 {
		t.Errorf("Arguments = %v, want empty map", canonical.Arguments)
	}
}

func TestOpenAIAdapter_Decode_NoToolCalls(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for messages without tool_calls")
	}
	if !strings.Contains(err.Error(), "no tool_calls found") {
		t.Errorf("error = %q, want to contain 'no tool_calls found'", err.Error())
	}
}

func TestOpenAIAdapter_Decode_InvalidJSON(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
	if !strings.Contains(err.Error(), "parse json") {
		t.Errorf("error = %q, want to contain 'parse json'", err.Error())
	}
}

func TestOpenAIAdapter_Decode_InvalidArguments(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	body := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_bad",
						"type": "function",
						"function": {
							"name": "test",
							"arguments": "not json"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := a.Decode(req)
	if err == nil {
		t.Fatal("expected error for invalid arguments json")
	}
	if !strings.Contains(err.Error(), "parse arguments") {
		t.Errorf("error = %q, want to contain 'parse arguments'", err.Error())
	}
}

func TestOpenAIAdapter_Decode_LatestAssistantMessage(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	// Multiple assistant messages — should decode the last one.
	body := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_old",
						"type": "function",
						"function": {
							"name": "old_tool",
							"arguments": "{}"
						}
					}
				]
			},
			{"role": "tool", "tool_call_id": "call_old", "content": "old result"},
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_new",
						"type": "function",
						"function": {
							"name": "new_tool",
							"arguments": "{\"key\":\"value\"}"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if canonical.ToolName != "new_tool" {
		t.Errorf("ToolName = %q, want new_tool (latest assistant message)", canonical.ToolName)
	}
	if canonical.RequestID != "call_new" {
		t.Errorf("RequestID = %q, want call_new", canonical.RequestID)
	}
}

func TestOpenAIAdapter_Encode_Success(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter(WithModelName("test-model"))
	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "The weather in SF is 18C."},
		},
		RequestID: "call_abc123",
	}

	body, ct, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if completion.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", completion.Object)
	}
	if completion.Model != "test-model" {
		t.Errorf("model = %q, want test-model", completion.Model)
	}
	if len(completion.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(completion.Choices))
	}

	choice := completion.Choices[0]
	if choice.Message.Role != "tool" {
		t.Errorf("role = %q, want tool", choice.Message.Role)
	}
	if choice.Message.ToolCallID != "call_abc123" {
		t.Errorf("tool_call_id = %q, want call_abc123", choice.Message.ToolCallID)
	}
	if choice.FinishReason != "stop" {
		t.Errorf("finish_reason = %q, want stop", choice.FinishReason)
	}

	// Content should be a JSON string containing the text.
	var content string
	if err := json.Unmarshal(choice.Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content != "The weather in SF is 18C." {
		t.Errorf("content = %q, want 'The weather in SF is 18C.'", content)
	}
}

func TestOpenAIAdapter_Encode_Error(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrNotFound,
			Message: "tool not found: nonexistent",
		},
		RequestID: "call_err",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(completion.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(completion.Choices))
	}

	var content string
	if err := json.Unmarshal(completion.Choices[0].Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if !strings.Contains(content, "Error:") {
		t.Errorf("content = %q, want to contain 'Error:'", content)
	}
	if !strings.Contains(content, "tool not found") {
		t.Errorf("content = %q, want to contain 'tool not found'", content)
	}
}

func TestOpenAIAdapter_Encode_MultipleContentParts(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "Part 1"},
			{Type: ContentTypeText, Text: "Part 2"},
			{Type: ContentTypeJSON, JSON: map[string]string{"key": "value"}},
		},
		RequestID: "call_multi",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var content string
	if err := json.Unmarshal(completion.Choices[0].Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}

	if !strings.Contains(content, "Part 1") {
		t.Errorf("content missing 'Part 1': %q", content)
	}
	if !strings.Contains(content, "Part 2") {
		t.Errorf("content missing 'Part 2': %q", content)
	}
	if !strings.Contains(content, "key") {
		t.Errorf("content missing JSON part: %q", content)
	}
}

func TestOpenAIAdapter_Encode_EmptyRequestID(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	resp := &CanonicalResponse{
		Success:   true,
		Content:   []ContentPart{{Type: ContentTypeText, Text: "ok"}},
		RequestID: "",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Should use fallback ID.
	if completion.Choices[0].Message.ToolCallID != "call_unknown" {
		t.Errorf("tool_call_id = %q, want call_unknown", completion.Choices[0].Message.ToolCallID)
	}
}

func TestOpenAIAdapter_Encode_EmptyContent(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	resp := &CanonicalResponse{
		Success:   true,
		Content:   nil,
		RequestID: "call_empty",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var content string
	if err := json.Unmarshal(completion.Choices[0].Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content != "" {
		t.Errorf("content = %q, want empty string", content)
	}
}

func TestOpenAIAdapter_Encode_DataContent(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeData, Data: []byte("binary"), MimeType: "application/octet-stream"},
			{Type: ContentTypeImage, MimeType: "image/png"},
		},
		RequestID: "call_data",
	}

	body, _, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var completion openaiChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var content string
	if err := json.Unmarshal(completion.Choices[0].Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if !strings.Contains(content, "binary data") {
		t.Errorf("content = %q, want to mention binary data", content)
	}
	if !strings.Contains(content, "image/png") {
		t.Errorf("content = %q, want to mention image/png", content)
	}
}

func TestOpenAIAdapter_DefaultModelName(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter()
	if a.modelName != "mcpkit-gateway" {
		t.Errorf("default modelName = %q, want mcpkit-gateway", a.modelName)
	}
}

func TestOpenAIAdapter_CustomModelName(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter(WithModelName("custom-model"))
	if a.modelName != "custom-model" {
		t.Errorf("modelName = %q, want custom-model", a.modelName)
	}
}

func TestOpenAIAdapter_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time check that OpenAIAdapter satisfies the Adapter interface.
	var _ Adapter = (*OpenAIAdapter)(nil)
}

func TestOpenAIAdapter_RoundTrip(t *testing.T) {
	t.Parallel()

	a := NewOpenAIAdapter(WithModelName("roundtrip-model"))

	// Decode a request.
	body := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_rt",
						"type": "function",
						"function": {
							"name": "echo",
							"arguments": "{\"message\":\"hello world\"}"
						}
					}
				]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := a.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// Simulate a successful tool result.
	resp := &CanonicalResponse{
		Success:   true,
		Content:   []ContentPart{{Type: ContentTypeText, Text: "echo: hello world"}},
		RequestID: canonical.RequestID,
	}

	// Encode the response.
	encoded, ct, err := a.Encode(resp)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	// Verify the round-trip produces valid OpenAI JSON.
	var completion openaiChatCompletion
	if err := json.Unmarshal(encoded, &completion); err != nil {
		t.Fatalf("unmarshal round-trip response: %v", err)
	}

	if completion.Model != "roundtrip-model" {
		t.Errorf("model = %q, want roundtrip-model", completion.Model)
	}
	if completion.Choices[0].Message.ToolCallID != "call_rt" {
		t.Errorf("tool_call_id = %q, want call_rt", completion.Choices[0].Message.ToolCallID)
	}

	var content string
	if err := json.Unmarshal(completion.Choices[0].Message.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content != "echo: hello world" {
		t.Errorf("content = %q, want 'echo: hello world'", content)
	}
}

func TestParseArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{"valid object", `{"key":"val","num":42}`, 2, false},
		{"empty object", `{}`, 0, false},
		{"empty string", ``, 0, false},
		{"whitespace", `  `, 0, false},
		{"invalid json", `not json`, 0, true},
		{"array not object", `[1,2,3]`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args, err := parseArguments(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArguments(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(args) != tt.wantLen {
				t.Errorf("parseArguments(%q) len = %d, want %d", tt.input, len(args), tt.wantLen)
			}
		})
	}
}

func TestBuildContentString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		resp *CanonicalResponse
		want string
	}{
		{
			"error response",
			&CanonicalResponse{
				Success: false,
				Error:   &ErrorInfo{Code: ErrInternal, Message: "boom"},
			},
			"Error: boom",
		},
		{
			"single text",
			&CanonicalResponse{
				Success: true,
				Content: []ContentPart{{Type: ContentTypeText, Text: "hello"}},
			},
			"hello",
		},
		{
			"empty content",
			&CanonicalResponse{
				Success: true,
				Content: nil,
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildContentString(tt.resp)
			if got != tt.want {
				t.Errorf("buildContentString() = %q, want %q", got, tt.want)
			}
		})
	}
}
