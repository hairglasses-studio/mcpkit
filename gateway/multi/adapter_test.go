package multi

import (
	"testing"
)

func TestProtocolConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		protocol Protocol
		want     string
	}{
		{ProtocolMCP, "mcp"},
		{ProtocolA2A, "a2a"},
		{ProtocolOpenAI, "openai"},
		{ProtocolUnknown, "unknown"},
	}
	for _, tt := range tests {
		if string(tt.protocol) != tt.want {
			t.Errorf("Protocol constant %q = %q, want %q", tt.protocol, string(tt.protocol), tt.want)
		}
	}
}

func TestConfidenceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		conf Confidence
		want string
	}{
		{ConfidenceLow, "low"},
		{ConfidenceMedium, "medium"},
		{ConfidenceHigh, "high"},
		{ConfidenceDefinitive, "definitive"},
		{Confidence(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.conf.String(); got != tt.want {
			t.Errorf("Confidence(%d).String() = %q, want %q", tt.conf, got, tt.want)
		}
	}
}

func TestConfidenceOrdering(t *testing.T) {
	t.Parallel()

	if ConfidenceLow >= ConfidenceMedium {
		t.Error("Low should be less than Medium")
	}
	if ConfidenceMedium >= ConfidenceHigh {
		t.Error("Medium should be less than High")
	}
	if ConfidenceHigh >= ConfidenceDefinitive {
		t.Error("High should be less than Definitive")
	}
}

func TestCanonicalRequest_Fields(t *testing.T) {
	t.Parallel()

	req := &CanonicalRequest{
		Protocol:  ProtocolMCP,
		ToolName:  "test_tool",
		Arguments: map[string]any{"key": "value"},
		RequestID: "req-123",
		Auth: &AuthContext{
			Token:   "tok",
			Subject: "user@example.com",
			Scopes:  []string{"read", "write"},
		},
		Metadata: map[string]string{"session": "abc"},
	}

	if req.Protocol != ProtocolMCP {
		t.Errorf("Protocol = %q, want mcp", req.Protocol)
	}
	if req.ToolName != "test_tool" {
		t.Errorf("ToolName = %q, want test_tool", req.ToolName)
	}
	if req.Arguments["key"] != "value" {
		t.Error("Arguments missing expected key")
	}
	if req.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want req-123", req.RequestID)
	}
	if req.Auth.Subject != "user@example.com" {
		t.Errorf("Auth.Subject = %q", req.Auth.Subject)
	}
	if len(req.Auth.Scopes) != 2 {
		t.Errorf("Auth.Scopes len = %d, want 2", len(req.Auth.Scopes))
	}
	if req.Metadata["session"] != "abc" {
		t.Error("Metadata missing session key")
	}
}

func TestCanonicalResponse_Success(t *testing.T) {
	t.Parallel()

	resp := &CanonicalResponse{
		Success: true,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "hello"},
		},
		RequestID: "req-1",
	}

	if !resp.Success {
		t.Error("expected success")
	}
	if len(resp.Content) != 1 {
		t.Fatal("expected 1 content part")
	}
	if resp.Content[0].Text != "hello" {
		t.Errorf("Content[0].Text = %q, want hello", resp.Content[0].Text)
	}
}

func TestCanonicalResponse_Error(t *testing.T) {
	t.Parallel()

	resp := &CanonicalResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    ErrNotFound,
			Message: "tool not found",
		},
		RequestID: "req-2",
	}

	if resp.Success {
		t.Error("expected failure")
	}
	if resp.Error == nil {
		t.Fatal("expected error info")
	}
	if resp.Error.Code != ErrNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, ErrNotFound)
	}
}

func TestErrorCodeValues(t *testing.T) {
	t.Parallel()

	// Ensure error codes are distinct.
	codes := []ErrorCode{
		ErrNone, ErrInvalidParams, ErrNotFound,
		ErrUnauthorized, ErrForbidden, ErrRateLimit,
		ErrTimeout, ErrInternal,
	}
	seen := make(map[ErrorCode]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate error code: %d", c)
		}
		seen[c] = true
	}
}

func TestContentTypes(t *testing.T) {
	t.Parallel()

	if ContentTypeText != "text" {
		t.Errorf("ContentTypeText = %q", ContentTypeText)
	}
	if ContentTypeJSON != "json" {
		t.Errorf("ContentTypeJSON = %q", ContentTypeJSON)
	}
	if ContentTypeImage != "image" {
		t.Errorf("ContentTypeImage = %q", ContentTypeImage)
	}
	if ContentTypeData != "data" {
		t.Errorf("ContentTypeData = %q", ContentTypeData)
	}
}
