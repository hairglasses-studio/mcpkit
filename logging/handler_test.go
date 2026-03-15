package logging

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type logEntry struct {
	Level  string
	Logger string
	Data   any
}

type mockSender struct {
	mu      sync.Mutex
	entries []logEntry
}

func (m *mockSender) SendLog(_ context.Context, level, logger string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, logEntry{Level: level, Logger: logger, Data: data})
	return nil
}

func (m *mockSender) getEntries() []logEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]logEntry(nil), m.entries...)
}

func TestHandler_BasicLevels(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender, Config{LoggerName: "test", MinLevel: DefaultMinLevel})
	logger := slog.New(h)

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	entries := sender.getEntries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	expected := []struct {
		level  string
		logger string
	}{
		{"debug", "test"},
		{"info", "test"},
		{"warning", "test"},
		{"error", "test"},
	}

	for i, e := range expected {
		if entries[i].Level != e.level {
			t.Errorf("entry %d: level = %q, want %q", i, entries[i].Level, e.level)
		}
		if entries[i].Logger != e.logger {
			t.Errorf("entry %d: logger = %q, want %q", i, entries[i].Logger, e.logger)
		}
	}
}

func TestHandler_MinLevel(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender, Config{MinLevel: slog.LevelWarn})
	logger := slog.New(h)

	logger.Debug("filtered")
	logger.Info("filtered")
	logger.Warn("included")
	logger.Error("included")

	entries := sender.getEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Level != "warning" {
		t.Errorf("first entry level = %q, want %q", entries[0].Level, "warning")
	}
	if entries[1].Level != "error" {
		t.Errorf("second entry level = %q, want %q", entries[1].Level, "error")
	}
}

func TestHandler_Attributes(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender)
	logger := slog.New(h)

	logger.Info("test", "key1", "val1", "key2", 42)

	entries := sender.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	data, ok := entries[0].Data.(map[string]any)
	if !ok {
		t.Fatalf("data is not map[string]any: %T", entries[0].Data)
	}
	if data["message"] != "test" {
		t.Errorf("message = %v, want %q", data["message"], "test")
	}
	if data["key1"] != "val1" {
		t.Errorf("key1 = %v, want %q", data["key1"], "val1")
	}
	if v, ok := data["key2"].(int64); !ok || v != 42 {
		t.Errorf("key2 = %v (%T), want 42", data["key2"], data["key2"])
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender)
	logger := slog.New(h.WithAttrs([]slog.Attr{slog.String("service", "myapp")}))

	logger.Info("hello")

	entries := sender.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	data := entries[0].Data.(map[string]any)
	if data["service"] != "myapp" {
		t.Errorf("service = %v, want %q", data["service"], "myapp")
	}
}

func TestHandler_WithGroup(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender)
	logger := slog.New(h.WithGroup("request"))

	logger.Info("incoming", "method", "GET", "path", "/api")

	entries := sender.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	data := entries[0].Data.(map[string]any)

	reqGroup, ok := data["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request group, got %v", data)
	}
	if reqGroup["method"] != "GET" {
		t.Errorf("method = %v, want %q", reqGroup["method"], "GET")
	}
	if reqGroup["path"] != "/api" {
		t.Errorf("path = %v, want %q", reqGroup["path"], "/api")
	}
}

func TestHandler_RateLimit(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender, Config{RateLimit: 50 * time.Millisecond})
	logger := slog.New(h)

	// First message should go through
	logger.Info("first")
	// Immediate second message should be dropped
	logger.Info("second")

	entries := sender.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (rate limited), got %d", len(entries))
	}
	data := entries[0].Data.(map[string]any)
	if data["message"] != "first" {
		t.Errorf("message = %v, want %q", data["message"], "first")
	}

	// After waiting, next message should go through
	time.Sleep(60 * time.Millisecond)
	logger.Info("third")

	entries = sender.getEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after wait, got %d", len(entries))
	}
}

func TestHandler_DefaultConfig(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender)

	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("default config should enable debug level")
	}
	if h.config.LoggerName != "" {
		t.Errorf("default logger name should be empty, got %q", h.config.LoggerName)
	}
	if h.config.RateLimit != 0 {
		t.Errorf("default rate limit should be 0, got %v", h.config.RateLimit)
	}
}

func TestHandler_NestedGroups(t *testing.T) {
	sender := &mockSender{}
	h := NewHandler(sender)
	logger := slog.New(h.WithGroup("outer").WithGroup("inner"))

	logger.Info("nested", "key", "value")

	entries := sender.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	data := entries[0].Data.(map[string]any)
	outer, ok := data["outer"].(map[string]any)
	if !ok {
		t.Fatalf("expected outer group, got %v", data)
	}
	inner, ok := outer["inner"].(map[string]any)
	if !ok {
		t.Fatalf("expected inner group, got %v", outer)
	}
	if inner["key"] != "value" {
		t.Errorf("key = %v, want %q", inner["key"], "value")
	}
}

func TestSlogToMCPLevel(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug - 4, "debug"},
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warning"},
		{slog.LevelError, "error"},
		{slog.LevelError + 4, "error"},
	}
	for _, tt := range tests {
		got := slogToMCPLevel(tt.level)
		if got != tt.want {
			t.Errorf("slogToMCPLevel(%v) = %q, want %q", tt.level, got, tt.want)
		}
	}
}
