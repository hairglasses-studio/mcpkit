package multi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestA2AAdapter_Protocol(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()
	if adapter.Protocol() != ProtocolA2A {
		t.Errorf("Protocol() = %q, want %q", adapter.Protocol(), ProtocolA2A)
	}
}

func TestA2AAdapter_Detect_JSONRPCMethods(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	methods := []string{
		"a2a.sendMessage",
		"a2a.sendStreamingMessage",
		"a2a.getTask",
		"a2a.cancelTask",
		"a2a.listTasks",
		"a2a.getExtendedAgentCard",
		"a2a.subscribeToTask",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			body := `{"jsonrpc":"2.0","method":"` + method + `","id":1}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			matches, conf := adapter.Detect(req, []byte(body))
			if !matches {
				t.Errorf("Detect(%q) matches = false, want true", method)
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("Detect(%q) confidence = %v, want definitive", method, conf)
			}
		})
	}
}

func TestA2AAdapter_Detect_WellKnownPath(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	paths := []string{
		"/.well-known/agent-card.json",
		"/agent-card:extended",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path, nil)
			matches, conf := adapter.Detect(req, nil)
			if !matches {
				t.Errorf("Detect(path=%q) matches = false, want true", path)
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("Detect(path=%q) confidence = %v, want definitive", path, conf)
			}
		})
	}
}

func TestA2AAdapter_Detect_PathPrefix(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	tests := []struct {
		path string
		want bool
	}{
		{"/a2a", true},
		{"/a2a/", true},
		{"/a2a/stream", true},
		{"/mcp", false},
		{"/openai/v1", false},
		{"/random", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			matches, conf := adapter.Detect(req, nil)
			if matches != tt.want {
				t.Errorf("Detect(path=%q) matches = %v, want %v", tt.path, matches, tt.want)
			}
			if tt.want && conf != ConfidenceHigh {
				t.Errorf("Detect(path=%q) confidence = %v, want high", tt.path, conf)
			}
		})
	}
}

func TestA2AAdapter_Detect_RejectsNonA2A(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	tests := []struct {
		name string
		path string
		body string
	}{
		{
			"MCP method",
			"/",
			`{"jsonrpc":"2.0","method":"tools/call","id":1}`,
		},
		{
			"OpenAI structure",
			"/",
			`{"tool_calls":[{"function":{"name":"test"}}]}`,
		},
		{
			"empty body",
			"/",
			"",
		},
		{
			"MCP path",
			"/mcp",
			"",
		},
		{
			"random path",
			"/api/v1/invoke",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			matches, _ := adapter.Detect(req, []byte(tt.body))
			if matches {
				t.Errorf("Detect should not match for %q", tt.name)
			}
		})
	}
}

func TestA2AAdapter_Decode_SendMessage_DataPart(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	// Build an a2a.sendMessage request with a DataPart containing skill and arguments.
	body := `{
		"jsonrpc": "2.0",
		"method": "a2a.sendMessage",
		"id": "req-42",
		"params": {
			"message": {
				"messageId": "msg-1",
				"role": "ROLE_USER",
				"parts": [
					{
						"data": {
							"skill": "system_info",
							"arguments": {
								"format": "json",
								"verbose": true
							}
						}
					}
				]
			}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := adapter.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.Protocol != ProtocolA2A {
		t.Errorf("Protocol = %q, want a2a", canonical.Protocol)
	}
	if canonical.ToolName != "system_info" {
		t.Errorf("ToolName = %q, want system_info", canonical.ToolName)
	}
	if canonical.RequestID != "req-42" {
		t.Errorf("RequestID = %q, want req-42", canonical.RequestID)
	}
	if canonical.Arguments["format"] != "json" {
		t.Errorf("Arguments[format] = %v, want json", canonical.Arguments["format"])
	}
	if canonical.Arguments["verbose"] != true {
		t.Errorf("Arguments[verbose] = %v, want true", canonical.Arguments["verbose"])
	}
	if canonical.Metadata["a2a.method"] != "a2a.sendMessage" {
		t.Errorf("Metadata[a2a.method] = %q, want a2a.sendMessage", canonical.Metadata["a2a.method"])
	}
}

func TestA2AAdapter_Decode_SendMessage_TextPart(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	// A text part with embedded JSON containing a skill reference.
	body := `{
		"jsonrpc": "2.0",
		"method": "a2a.sendMessage",
		"id": 7,
		"params": {
			"message": {
				"messageId": "msg-2",
				"role": "ROLE_USER",
				"parts": [
					{
						"text": "{\"skill\": \"echo_tool\", \"arguments\": {\"message\": \"hello\"}}"
					}
				]
			}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := adapter.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.ToolName != "echo_tool" {
		t.Errorf("ToolName = %q, want echo_tool", canonical.ToolName)
	}
	if canonical.Arguments["message"] != "hello" {
		t.Errorf("Arguments[message] = %v, want hello", canonical.Arguments["message"])
	}
	// Numeric JSON-RPC ID should be extracted as a string.
	if canonical.RequestID != "7" {
		t.Errorf("RequestID = %q, want 7", canonical.RequestID)
	}
}

func TestA2AAdapter_Decode_NonSendMessage(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	body := `{
		"jsonrpc": "2.0",
		"method": "a2a.getTask",
		"id": "req-99",
		"params": {"id": "task-abc"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	canonical, err := adapter.Decode(req)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if canonical.ToolName != "" {
		t.Errorf("ToolName = %q, want empty for non-sendMessage", canonical.ToolName)
	}
	if canonical.Metadata["a2a.method"] != "a2a.getTask" {
		t.Errorf("Metadata[a2a.method] = %q, want a2a.getTask", canonical.Metadata["a2a.method"])
	}
}

func TestA2AAdapter_Decode_InvalidJSON(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	_, err := adapter.Decode(req)
	if err == nil {
		t.Fatal("Decode() should fail for invalid JSON")
	}
}

func TestA2AAdapter_Decode_NoMessage(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	body := `{
		"jsonrpc": "2.0",
		"method": "a2a.sendMessage",
		"id": "req-1",
		"params": {}
	}`

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := adapter.Decode(req)
	if err == nil {
		t.Fatal("Decode() should fail when message is nil")
	}
}

func TestA2AAdapter_Decode_NoSkillInParts(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	body := `{
		"jsonrpc": "2.0",
		"method": "a2a.sendMessage",
		"id": "req-1",
		"params": {
			"message": {
				"messageId": "msg-1",
				"role": "ROLE_USER",
				"parts": [
					{"text": "just plain text without skill info"}
				]
			}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := adapter.Decode(req)
	if err == nil {
		t.Fatal("Decode() should fail when no skill identifier is found")
	}
}

func TestA2AAdapter_Encode_Success(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "operation completed successfully"},
		},
		RequestID: "req-42",
		Metadata: map[string]string{
			"a2a.taskId":    "task-123",
			"a2a.contextId": "ctx-456",
		},
	}

	body, contentType, err := adapter.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("contentType = %q, want application/json", contentType)
	}

	// Parse the JSON-RPC response.
	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			ID        string `json:"id"`
			ContextID string `json:"contextId"`
			Status    struct {
				State string `json:"state"`
			} `json:"status"`
			Artifacts []struct {
				ID    string `json:"artifactId"`
				Parts []struct {
					Text string `json:"text,omitempty"`
				} `json:"parts"`
			} `json:"artifacts"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", rpcResp.JSONRPC)
	}
	if rpcResp.ID != "req-42" {
		t.Errorf("id = %q, want req-42", rpcResp.ID)
	}
	if rpcResp.Result.ID != "task-123" {
		t.Errorf("result.id = %q, want task-123", rpcResp.Result.ID)
	}
	if rpcResp.Result.ContextID != "ctx-456" {
		t.Errorf("result.contextId = %q, want ctx-456", rpcResp.Result.ContextID)
	}
	if rpcResp.Result.Status.State != "TASK_STATE_COMPLETED" {
		t.Errorf("result.status.state = %q, want TASK_STATE_COMPLETED", rpcResp.Result.Status.State)
	}
	if len(rpcResp.Result.Artifacts) != 1 {
		t.Fatalf("artifacts count = %d, want 1", len(rpcResp.Result.Artifacts))
	}
	if len(rpcResp.Result.Artifacts[0].Parts) != 1 {
		t.Fatalf("artifact parts count = %d, want 1", len(rpcResp.Result.Artifacts[0].Parts))
	}
	if rpcResp.Result.Artifacts[0].Parts[0].Text != "operation completed successfully" {
		t.Errorf("artifact text = %q, want 'operation completed successfully'",
			rpcResp.Result.Artifacts[0].Parts[0].Text)
	}
}

func TestA2AAdapter_Encode_Error(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrNotFound,
			Message: "tool 'nonexistent' not found",
		},
		RequestID: "req-err",
	}

	body, contentType, err := adapter.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("contentType = %q, want application/json", contentType)
	}

	var rpcResp struct {
		Result struct {
			Status struct {
				State   string `json:"state"`
				Message *struct {
					Parts []struct {
						Text string `json:"text,omitempty"`
					} `json:"parts"`
				} `json:"message"`
			} `json:"status"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rpcResp.Result.Status.State != "TASK_STATE_FAILED" {
		t.Errorf("state = %q, want TASK_STATE_FAILED", rpcResp.Result.Status.State)
	}
	if rpcResp.Result.Status.Message == nil {
		t.Fatal("expected status message for failed task")
	}
	if len(rpcResp.Result.Status.Message.Parts) == 0 {
		t.Fatal("expected at least one part in status message")
	}
	if !strings.Contains(rpcResp.Result.Status.Message.Parts[0].Text, "nonexistent") {
		t.Errorf("error message = %q, want to contain 'nonexistent'",
			rpcResp.Result.Status.Message.Parts[0].Text)
	}
}

func TestA2AAdapter_Encode_GeneratesIDsWhenMissing(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "ok"},
		},
		RequestID: "req-1",
		// No metadata with task/context IDs.
	}

	body, _, err := adapter.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp struct {
		Result struct {
			ID        string `json:"id"`
			ContextID string `json:"contextId"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rpcResp.Result.ID == "" {
		t.Error("expected generated task ID")
	}
	if rpcResp.Result.ContextID == "" {
		t.Error("expected generated context ID")
	}
}

func TestA2AAdapter_Encode_MultipleContentParts(t *testing.T) {
	t.Parallel()

	adapter := NewA2AAdapter()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "part 1"},
			{Type: ContentTypeJSON, JSON: map[string]any{"key": "value"}},
		},
		RequestID: "req-multi",
	}

	body, _, err := adapter.Encode(resp)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var rpcResp struct {
		Result struct {
			Artifacts []struct {
				Parts []json.RawMessage `json:"parts"`
			} `json:"artifacts"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(rpcResp.Result.Artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(rpcResp.Result.Artifacts))
	}
	if len(rpcResp.Result.Artifacts[0].Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(rpcResp.Result.Artifacts[0].Parts))
	}
}

func TestA2AAdapter_RoundTrip_ImplementsAdapter(t *testing.T) {
	t.Parallel()

	// Verify the adapter can be registered with the Router.
	var adapter Adapter = NewA2AAdapter()
	if adapter.Protocol() != ProtocolA2A {
		t.Errorf("Protocol() = %q", adapter.Protocol())
	}
}

func TestExtractRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"string id", `"req-42"`, "req-42"},
		{"numeric id", `7`, "7"},
		{"null id", `null`, "null"},
		{"empty", ``, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractRequestID(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("extractRequestID(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractSkillFromMessage_EmptyParts(t *testing.T) {
	t.Parallel()

	_, _, err := extractSkillFromMessage(nil)
	if err == nil {
		t.Error("expected error for nil message")
	}
}

func TestCanonicalPartToA2A(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		part    ContentPart
		wantNil bool
	}{
		{
			"text part",
			ContentPart{Type: ContentTypeText, Text: "hello"},
			false,
		},
		{
			"json part",
			ContentPart{Type: ContentTypeJSON, JSON: map[string]any{"k": "v"}},
			false,
		},
		{
			"data part with bytes",
			ContentPart{Type: ContentTypeData, Data: []byte("raw")},
			false,
		},
		{
			"data part without bytes",
			ContentPart{Type: ContentTypeData},
			true,
		},
		{
			"image part with bytes",
			ContentPart{Type: ContentTypeImage, Data: []byte("img"), MimeType: "image/png"},
			false,
		},
		{
			"image part without bytes",
			ContentPart{Type: ContentTypeImage, MimeType: "image/png"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := canonicalPartToA2A(tt.part)
			if (got == nil) != tt.wantNil {
				t.Errorf("canonicalPartToA2A() nil = %v, wantNil = %v", got == nil, tt.wantNil)
			}
		})
	}
}
