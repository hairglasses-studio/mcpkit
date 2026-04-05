package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code int
		want int
	}{
		{"ParseError", CodeParseError, -32700},
		{"InvalidRequest", CodeInvalidRequest, -32600},
		{"MethodNotFound", CodeMethodNotFound, -32601},
		{"InvalidParams", CodeInvalidParams, -32602},
		{"InternalError", CodeInternalError, -32603},
		{"RequestCancelled", CodeRequestCancelled, -32800},
		{"ResourceNotFound", CodeResourceNotFound, -32002},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.code != tt.want {
				t.Errorf("Code%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	t.Parallel()

	e := NewError(CodeMethodNotFound, "tools/call not found")
	got := e.Error()
	want := "jsonrpc error -32601: tools/call not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_ErrorWithData(t *testing.T) {
	t.Parallel()

	e := NewErrorWithData(CodeInvalidParams, "bad params", map[string]string{"field": "name"})
	got := e.Error()
	if got == "" {
		t.Error("Error() returned empty string")
	}
	// Should contain "data:"
	if !contains(got, "data:") {
		t.Errorf("Error() = %q, expected to contain 'data:'", got)
	}
}

func TestError_Is(t *testing.T) {
	t.Parallel()

	e := NewError(CodeMethodNotFound, "custom message")
	if !errors.Is(e, ErrMethodNotFound) {
		t.Error("expected errors.Is(custom, ErrMethodNotFound) to be true")
	}

	if errors.Is(e, ErrParseError) {
		t.Error("expected errors.Is(custom, ErrParseError) to be false")
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := []*Error{
		ErrParseError,
		ErrInvalidRequest,
		ErrMethodNotFound,
		ErrInvalidParams,
		ErrInternalError,
		ErrRequestCancelled,
		ErrResourceNotFound,
	}

	for _, s := range sentinels {
		t.Run(CodeName(s.Code), func(t *testing.T) {
			t.Parallel()
			// Each sentinel should be its own error.
			if s.Error() == "" {
				t.Error("sentinel Error() returned empty")
			}
			// Sentinel should match itself.
			if !errors.Is(s, s) {
				t.Error("sentinel does not match itself")
			}
		})
	}
}

func TestIsStandardCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want bool
	}{
		{CodeParseError, true},
		{CodeInvalidRequest, true},
		{CodeMethodNotFound, true},
		{CodeInvalidParams, true},
		{CodeInternalError, true},
		{CodeRequestCancelled, true},
		{CodeResourceNotFound, true},
		{-32000, false},
		{0, false},
		{200, false},
	}

	for _, tt := range tests {
		got := IsStandardCode(tt.code)
		if got != tt.want {
			t.Errorf("IsStandardCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestCodeName(t *testing.T) {
	t.Parallel()

	if got := CodeName(CodeParseError); got != "ParseError" {
		t.Errorf("CodeName(%d) = %q, want ParseError", CodeParseError, got)
	}
	if got := CodeName(0); got != "Unknown" {
		t.Errorf("CodeName(0) = %q, want Unknown", got)
	}
}

func TestRequest_IsNotification(t *testing.T) {
	t.Parallel()

	notif := &Request{JSONRPC: JSONRPCVersion, Method: "notifications/initialized"}
	if !notif.IsNotification() {
		t.Error("expected request without ID to be a notification")
	}

	req := &Request{JSONRPC: JSONRPCVersion, ID: 1, Method: "tools/call"}
	if req.IsNotification() {
		t.Error("expected request with ID to not be a notification")
	}
}

func TestNewResponse(t *testing.T) {
	t.Parallel()

	resp := NewResponse(42, map[string]string{"status": "ok"})
	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
	if resp.ID != 42 {
		t.Errorf("ID = %v, want 42", resp.ID)
	}
	if resp.Error != nil {
		t.Error("expected nil error on success response")
	}
}

func TestNewErrorResponse(t *testing.T) {
	t.Parallel()

	resp := NewErrorResponse(nil, ErrParseError)
	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
	if resp.ID != nil {
		t.Errorf("ID = %v, want nil", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if resp.Error.Code != CodeParseError {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeParseError)
	}
}

func TestResponse_JSON(t *testing.T) {
	t.Parallel()

	resp := NewErrorResponse(1, NewError(CodeMethodNotFound, "not found"))
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected error in decoded response")
	}
	if decoded.Error.Code != CodeMethodNotFound {
		t.Errorf("decoded error code = %d, want %d", decoded.Error.Code, CodeMethodNotFound)
	}
}

func TestRequest_JSON_Roundtrip(t *testing.T) {
	t.Parallel()

	original := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"test"}`),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Method != "tools/call" {
		t.Errorf("Method = %q, want tools/call", decoded.Method)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
