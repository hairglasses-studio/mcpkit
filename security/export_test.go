package security

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONLExporter_WritesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	exp := NewJSONLExporter(&buf)

	event := AuditEvent{
		ID:   "test-1",
		Type: AuditToolCall,
		User: "alice",
		Tool: "search",
	}
	if err := exp.Export(event); err != nil {
		t.Fatalf("Export: %v", err)
	}

	var parsed AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("parse JSONL: %v", err)
	}
	if parsed.User != "alice" {
		t.Errorf("user = %q, want alice", parsed.User)
	}
}

func TestJSONLExporter_MultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	exp := NewJSONLExporter(&buf)

	for range 5 {
		_ = exp.Export(AuditEvent{Type: AuditToolCall, User: "user"})
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("line count = %d, want 5", len(lines))
	}
}

func TestStreamExporter_WithFilter(t *testing.T) {
	var buf bytes.Buffer
	exp := NewStreamExporter(&buf, func(e AuditEvent) bool {
		return e.Type == AuditToolError
	})

	_ = exp.Export(AuditEvent{Type: AuditToolCall, User: "alice"})
	_ = exp.Export(AuditEvent{Type: AuditToolError, User: "bob"})
	_ = exp.Export(AuditEvent{Type: AuditToolSuccess, User: "carol"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 filtered line, got %d", len(lines))
	}
}

func TestStreamExporter_NoFilter(t *testing.T) {
	var buf bytes.Buffer
	exp := NewStreamExporter(&buf, nil)

	_ = exp.Export(AuditEvent{Type: AuditToolCall})
	_ = exp.Export(AuditEvent{Type: AuditToolError})
	_ = exp.Export(AuditEvent{Type: AuditToolSuccess})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestAuditLogger_WithExporter(t *testing.T) {
	var buf bytes.Buffer
	exp := NewJSONLExporter(&buf)

	logger := NewAuditLogger(AuditLoggerConfig{
		MaxEvents: 100,
		Exporters: []AuditExporter{exp},
	})

	logger.LogToolCall("alice", "tool_a", map[string]any{"key": "val"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 exported event, got %d", len(lines))
	}

	var parsed AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Tool != "tool_a" {
		t.Errorf("tool = %q, want tool_a", parsed.Tool)
	}
}

func TestExporter_Close(t *testing.T) {
	var buf bytes.Buffer
	exp := NewJSONLExporter(&buf)
	// Close on non-Closer writer should not error
	if err := exp.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
