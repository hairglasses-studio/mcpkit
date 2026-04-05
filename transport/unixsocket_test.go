package transport

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// --------------------------------------------------------------------------
// socketSession tests (unit tests for the session type)
// --------------------------------------------------------------------------

func TestSocketSession_NewSession(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("test-1")

	if ss.SessionID() != "test-1" {
		t.Errorf("expected session ID %q, got %q", "test-1", ss.SessionID())
	}
	if ss.Initialized() {
		t.Error("new session should not be initialized")
	}
}

func TestSocketSession_Initialize(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("init-test")

	ss.Initialize()
	if !ss.Initialized() {
		t.Error("expected session to be initialized after Initialize()")
	}

	// Idempotent.
	ss.Initialize()
	if !ss.Initialized() {
		t.Error("expected session to remain initialized")
	}
}

func TestSocketSession_NotificationChannel(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("notif-test")

	ch := ss.NotificationChannel()
	if ch == nil {
		t.Fatal("expected non-nil notification channel")
	}

	// Should be able to send without blocking (buffered channel).
	ch <- mcp.JSONRPCNotification{}
}

func TestSocketSession_LogLevel(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("log-test")

	// Default is error level.
	if ss.GetLogLevel() != mcp.LoggingLevelError {
		t.Errorf("expected default log level %v, got %v", mcp.LoggingLevelError, ss.GetLogLevel())
	}

	ss.SetLogLevel(mcp.LoggingLevelDebug)
	if ss.GetLogLevel() != mcp.LoggingLevelDebug {
		t.Errorf("expected log level %v, got %v", mcp.LoggingLevelDebug, ss.GetLogLevel())
	}
}

func TestSocketSession_ClientInfo(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("info-test")

	// Default should be zero value.
	info := ss.GetClientInfo()
	if info.Name != "" {
		t.Errorf("expected empty client name, got %q", info.Name)
	}

	ci := mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	ss.SetClientInfo(ci)
	got := ss.GetClientInfo()
	if got.Name != "test-client" || got.Version != "1.0.0" {
		t.Errorf("expected %v, got %v", ci, got)
	}
}

func TestSocketSession_ClientCapabilities(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("caps-test")

	// Default should be zero value.
	caps := ss.GetClientCapabilities()
	if caps.Sampling != nil {
		t.Errorf("expected nil sampling capability, got %v", caps.Sampling)
	}

	cc := mcp.ClientCapabilities{
		Sampling: &struct{}{},
	}
	ss.SetClientCapabilities(cc)
	got := ss.GetClientCapabilities()
	if got.Sampling == nil {
		t.Error("expected non-nil sampling capability after set")
	}
}

// --------------------------------------------------------------------------
// createJSONRPCError tests
// --------------------------------------------------------------------------

func TestCreateJSONRPCError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      any
		code    int
		message string
	}{
		{"nil ID", nil, -32700, "Parse error"},
		{"numeric ID", 42, -32600, "Invalid request"},
		{"string ID", "req-1", -32601, "Method not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := createJSONRPCError(tt.id, tt.code, tt.message)

			if err.JSONRPC != "2.0" {
				t.Errorf("expected jsonrpc 2.0, got %q", err.JSONRPC)
			}
			if err.ID != tt.id {
				t.Errorf("expected ID %v, got %v", tt.id, err.ID)
			}
			if err.Error.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, err.Error.Code)
			}
			if err.Error.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, err.Error.Message)
			}

			// Should be JSON-serializable.
			data, jsonErr := json.Marshal(err)
			if jsonErr != nil {
				t.Fatalf("Marshal: %v", jsonErr)
			}
			if !strings.Contains(string(data), `"jsonrpc":"2.0"`) {
				t.Errorf("expected JSON to contain jsonrpc field, got %s", data)
			}
		})
	}
}

// --------------------------------------------------------------------------
// WriteJSONLine tests
// --------------------------------------------------------------------------

func TestWriteJSONLine_WithMutex(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var mu sync.Mutex

	type msg struct {
		Method string `json:"method"`
	}

	err := WriteJSONLine(&buf, &mu, msg{Method: "test"})
	if err != nil {
		t.Fatalf("WriteJSONLine: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("expected newline terminator")
	}
	if !strings.Contains(line, `"method":"test"`) {
		t.Errorf("expected method in output, got %q", line)
	}
}

func TestWriteJSONLine_WithoutMutex(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := WriteJSONLine(&buf, nil, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("WriteJSONLine: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("expected newline terminator")
	}
	if !strings.Contains(line, `"key":"value"`) {
		t.Errorf("expected key in output, got %q", line)
	}
}

func TestWriteJSONLine_UnmarshalableValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Channels cannot be marshaled to JSON.
	err := WriteJSONLine(&buf, nil, make(chan int))
	if err == nil {
		t.Error("expected error for unmarshalable value")
	}
}

// --------------------------------------------------------------------------
// DefaultSocketPath tests
// --------------------------------------------------------------------------

func TestDefaultSocketPath_WithXDGRuntimeDir(t *testing.T) {
	// Save and restore original value.
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	path := DefaultSocketPath("test-server")
	expected := "/run/user/1000/mcpkit/test-server.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDefaultSocketPath_WithoutXDGRuntimeDir(t *testing.T) {
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Unsetenv("XDG_RUNTIME_DIR")
	path := DefaultSocketPath("test-server")
	expected := "/tmp/mcpkit/test-server.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDefaultSocketPath_EmptyName(t *testing.T) {
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	path := DefaultSocketPath("")
	expected := "/run/user/1000/mcpkit/.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
