package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditLogger_LogAndRetrieve(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolCall("alice", "tool_list", map[string]any{"category": "GPU"})
	logger.LogToolResult("alice", "tool_list", 50*time.Millisecond, nil)

	events := logger.GetRecentEvents(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != AuditToolCall {
		t.Errorf("first event type = %q, want %q", events[0].Type, AuditToolCall)
	}
	if events[1].Type != AuditToolSuccess {
		t.Errorf("second event type = %q, want %q", events[1].Type, AuditToolSuccess)
	}
}

func TestAuditLogger_ErrorEvents(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolResult("alice", "tool_delete", 10*time.Millisecond, fmt.Errorf("not found"))
	logger.LogToolResult("alice", "tool_list", 5*time.Millisecond, nil)
	logger.LogAccessDenied("unknown", "tool_delete", "insufficient permissions")

	errs := logger.GetErrorEvents(10)
	if len(errs) != 2 {
		t.Fatalf("expected 2 error events, got %d", len(errs))
	}
}

func TestAuditLogger_FilterByUser(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolCall("alice", "tool_a", nil)
	logger.LogToolCall("bob", "tool_b", nil)
	logger.LogToolCall("alice", "tool_c", nil)

	events := logger.GetEventsByUser("alice", 10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events for alice, got %d", len(events))
	}
}

func TestAuditLogger_FilterByTool(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolCall("alice", "tool_a", nil)
	logger.LogToolCall("bob", "tool_a", nil)
	logger.LogToolCall("alice", "tool_b", nil)

	events := logger.GetEventsByTool("tool_a", 10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events for tool_a, got %d", len(events))
	}
}

func TestAuditLogger_Stats(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolCall("alice", "tool_a", nil)
	logger.LogToolResult("alice", "tool_a", 100*time.Millisecond, nil)
	logger.LogToolResult("bob", "tool_b", 200*time.Millisecond, fmt.Errorf("fail"))

	stats := logger.GetStats()
	if stats.TotalEvents != 3 {
		t.Errorf("total events = %d, want 3", stats.TotalEvents)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("error count = %d, want 1", stats.ErrorCount)
	}
}

func TestAuditLogger_CircularBuffer(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 3})

	logger.LogToolCall("a", "t1", nil)
	logger.LogToolCall("b", "t2", nil)
	logger.LogToolCall("c", "t3", nil)
	logger.LogToolCall("d", "t4", nil)

	events := logger.GetRecentEvents(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events (circular), got %d", len(events))
	}
	if events[0].User != "b" {
		t.Errorf("oldest event user = %q, want %q", events[0].User, "b")
	}
}

func TestSanitizeAuditParams(t *testing.T) {
	params := map[string]any{
		"name":     "test",
		"password": "secret123",
		"api_key":  "key-abc",
		"category": "GPU",
	}

	sanitized := SanitizeAuditParams(params)
	if sanitized["name"] != "test" {
		t.Error("name should not be redacted")
	}
	if sanitized["password"] != "[REDACTED]" {
		t.Errorf("password should be redacted, got %v", sanitized["password"])
	}
	if sanitized["api_key"] != "[REDACTED]" {
		t.Errorf("api_key should be redacted, got %v", sanitized["api_key"])
	}
	if sanitized["category"] != "GPU" {
		t.Error("category should not be redacted")
	}
}

func TestSanitizeAuditParams_Nil(t *testing.T) {
	result := SanitizeAuditParams(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestSanitizeAuditParams_CaseInsensitive(t *testing.T) {
	params := map[string]any{
		"my_Password_field": "secret",
		"AUTH_header":       "bearer xyz",
		"user_token_value":  "tok123",
		"safe_field":        "ok",
	}
	sanitized := SanitizeAuditParams(params)

	if sanitized["my_Password_field"] != "[REDACTED]" {
		t.Errorf("my_Password_field should be redacted")
	}
	if sanitized["AUTH_header"] != "[REDACTED]" {
		t.Errorf("AUTH_header should be redacted")
	}
	if sanitized["safe_field"] != "ok" {
		t.Error("safe_field should not be redacted")
	}
}

func TestAuditLogger_WriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-audit.log")

	logger := NewAuditLogger(AuditLoggerConfig{LogFile: logFile, MaxEvents: 100})
	defer logger.Close()

	logger.LogToolCall("alice", "tool_a", map[string]any{"key": "value"})

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file should not be empty")
	}
	if !strings.Contains(string(data), "tool_a") {
		t.Error("log file should contain tool_a")
	}
}

func TestAuditLogger_GetRecentEvents_ZeroLimit(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogToolCall("a", "t1", nil)
	logger.LogToolCall("b", "t2", nil)

	events := logger.GetRecentEvents(0)
	if len(events) != 2 {
		t.Errorf("expected 2 events with limit=0, got %d", len(events))
	}
}

func TestAuditLogger_Stats_AccessDenied(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.LogAccessDenied("baduser", "secret_tool", "no perms")
	logger.LogAccessDenied("baduser", "other_tool", "no perms")

	stats := logger.GetStats()
	if stats.AccessDenied != 2 {
		t.Errorf("access denied = %d, want 2", stats.AccessDenied)
	}
}

func TestAuditLogger_ExplicitID(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	logger.Log(AuditEvent{
		ID:   "custom-id-123",
		Type: AuditToolCall,
		User: "alice",
		Tool: "tool_x",
	})

	events := logger.GetRecentEvents(1)
	if len(events) != 1 || events[0].ID != "custom-id-123" {
		t.Errorf("expected custom ID, got %v", events)
	}
}
