package slogcfg

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestInit_Defaults(t *testing.T) {
	var buf bytes.Buffer
	logger := Init(Config{Output: &buf, JSON: true})
	logger.Info("test message")
	if !strings.Contains(buf.String(), `"msg":"test message"`) {
		t.Errorf("expected JSON message, got: %s", buf.String())
	}
}

func TestInit_ServiceName(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{ServiceName: "my-server", Output: &buf, JSON: true})
	slog.Info("hello")
	if !strings.Contains(buf.String(), `"service":"my-server"`) {
		t.Errorf("expected service attribute, got: %s", buf.String())
	}
}

func TestInit_TextHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := Init(Config{Output: &buf, JSON: false})
	logger.Info("text mode")
	out := buf.String()
	// Text handler doesn't produce JSON braces.
	if strings.Contains(out, `{"`) {
		t.Errorf("expected text output, got JSON: %s", out)
	}
	if !strings.Contains(out, "text mode") {
		t.Errorf("expected message in output, got: %s", out)
	}
}

func TestInit_Level(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{Output: &buf, Level: slog.LevelWarn, JSON: true})
	slog.Info("should be suppressed")
	if buf.Len() > 0 {
		t.Errorf("expected Info to be suppressed at Warn level, got: %s", buf.String())
	}
	slog.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("expected Warn message, got: %s", buf.String())
	}
}

func TestInit_EmptyServiceName(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{Output: &buf, JSON: true})
	slog.Info("no service")
	if strings.Contains(buf.String(), `"service"`) {
		t.Errorf("expected no service attribute, got: %s", buf.String())
	}
}

func TestInit_ExtraHandler(t *testing.T) {
	var buf bytes.Buffer
	called := false
	Init(Config{
		Output: &buf,
		JSON:   true,
		ExtraHandler: func(h slog.Handler) slog.Handler {
			called = true
			return h // pass through
		},
	})
	if !called {
		t.Error("expected ExtraHandler to be called")
	}
}

func TestInit_ReturnsLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := Init(Config{Output: &buf, JSON: true})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}
