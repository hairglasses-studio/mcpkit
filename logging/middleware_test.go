package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddleware_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := Middleware(logger)
	td := registry.ToolDefinition{Category: "test"}
	handler := mw("my_tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatal("expected success result")
	}

	output := buf.String()
	if !strings.Contains(output, "tool invocation") {
		t.Errorf("expected log to contain 'tool invocation', got: %s", output)
	}
	if !strings.Contains(output, "my_tool") {
		t.Errorf("expected log to contain tool name, got: %s", output)
	}
	if !strings.Contains(output, "level=INFO") {
		t.Errorf("expected INFO level log, got: %s", output)
	}
}

func TestMiddleware_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := Middleware(logger)
	td := registry.ToolDefinition{Category: "api"}
	handler := mw("failing_tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, errors.New("connection refused")
	})

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	output := buf.String()
	if !strings.Contains(output, "level=ERROR") {
		t.Errorf("expected ERROR level log, got: %s", output)
	}
	if !strings.Contains(output, "tool invocation failed") {
		t.Errorf("expected failure message, got: %s", output)
	}
}

func TestMiddleware_ResultError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := Middleware(logger)
	td := registry.ToolDefinition{}
	handler := mw("erroring_tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult("invalid input"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Fatal("expected error result")
	}

	output := buf.String()
	if !strings.Contains(output, "level=WARN") {
		t.Errorf("expected WARN level log, got: %s", output)
	}
	if !strings.Contains(output, "tool returned error") {
		t.Errorf("expected result error message, got: %s", output)
	}
}

func TestMiddleware_DefaultCategory(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := Middleware(logger)
	td := registry.ToolDefinition{} // no category set
	handler := mw("tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	handler(context.Background(), registry.CallToolRequest{})

	output := buf.String()
	if !strings.Contains(output, "unknown") {
		t.Errorf("expected default category 'unknown', got: %s", output)
	}
}
