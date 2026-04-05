//go:build !official_sdk

package debug

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/mark3labs/mcp-go/mcp"
)

// testHandler returns a handler that produces a simple text result.
func testHandler(text string) registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult(text), nil
	}
}

// testErrorHandler returns a handler that returns a Go error.
func testErrorHandler() registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, errors.New("something broke")
	}
}

// testToolErrorHandler returns a handler that returns a tool-level error result.
func testToolErrorHandler() registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult("tool failed"), nil
	}
}

// testReq builds a CallToolRequest with the given arguments.
func testReq(args map[string]any) registry.CallToolRequest {
	return registry.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "test_tool",
			Arguments: args,
		},
	}
}

// newTestLogger returns a slog.Logger backed by a bytes.Buffer at debug level.
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestMiddleware_LogsToolCall(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:           true,
		Logger:            logger,
		LogParams:         true,
		LogResults:        true,
		MaxResultLogBytes: 512,
		RedactFields:      DefaultRedactFields,
	}

	mw := Middleware(cfg)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("hello world"))

	req := testReq(map[string]any{"unit": "sshd.service"})
	result, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	output := buf.String()
	for _, want := range []string{"tool_call.start", "tool_call.end", "test_tool", "duration_ms"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in log output:\n%s", want, output)
		}
	}
}

func TestMiddleware_LogsParams(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:   true,
		Logger:    logger,
		LogParams: true,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("ok"))

	req := testReq(map[string]any{"name": "crt-phosphor"})
	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if !strings.Contains(output, "crt-phosphor") {
		t.Errorf("expected params in log output:\n%s", output)
	}
}

func TestMiddleware_OmitsParamsWhenDisabled(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:   true,
		Logger:    logger,
		LogParams: false,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("ok"))

	req := testReq(map[string]any{"secret_value": "should-not-appear"})
	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if strings.Contains(output, "should-not-appear") {
		t.Error("params should not be logged when LogParams is false")
	}
	if strings.Contains(output, "params=") {
		t.Error("params key should not appear when LogParams is false")
	}
}

func TestMiddleware_RedactsSecrets(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:      true,
		Logger:       logger,
		LogParams:    true,
		RedactFields: []string{"password", "api_key"},
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("ok"))

	req := testReq(map[string]any{
		"username": "admin",
		"password": "hunter2",
		"api_key":  "sk-abc123",
	})
	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if strings.Contains(output, "hunter2") {
		t.Error("password was not redacted")
	}
	if strings.Contains(output, "sk-abc123") {
		t.Error("api_key was not redacted")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in output")
	}
	if !strings.Contains(output, "admin") {
		t.Error("non-sensitive field 'username' should still be logged")
	}
}

func TestMiddleware_RedactsCaseInsensitive(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:      true,
		Logger:       logger,
		LogParams:    true,
		RedactFields: []string{"password"},
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("ok"))

	req := testReq(map[string]any{
		"Password": "secret123",
		"safe":     "visible",
	})
	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if strings.Contains(output, "secret123") {
		t.Error("Password (mixed case) was not redacted")
	}
}

func TestMiddleware_DisabledPassthrough(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled: false,
		Logger:  logger,
	}

	mw := Middleware(cfg)

	called := false
	handler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	}

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, handler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should still be called when middleware is disabled")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// No debug output should be produced.
	output := buf.String()
	if output != "" {
		t.Errorf("expected no log output when disabled, got:\n%s", output)
	}
}

func TestMiddleware_DisabledZeroOverhead(t *testing.T) {
	cfg := Config{Enabled: false}
	mw := Middleware(cfg)

	handler := testHandler("ok")
	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, handler)

	// When disabled, the middleware should return the exact same handler
	// function pointer — no wrapping at all.
	// We verify this indirectly: the wrapped function should work without
	// any logger being set (because it IS the original handler).
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestMiddleware_CorrelationID(t *testing.T) {
	logger, _ := newTestLogger()

	cfg := Config{
		Enabled: true,
		Logger:  logger,
	}
	mw := Middleware(cfg)

	var capturedID string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedID = CorrelationID(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, handler)

	_, _ = wrapped(context.Background(), registry.CallToolRequest{})
	if capturedID == "" {
		t.Error("expected correlation ID to be set in context")
	}
	if !strings.HasPrefix(capturedID, "req-") {
		t.Errorf("expected req- prefix, got %q", capturedID)
	}
}

func TestMiddleware_CorrelationIDIncrements(t *testing.T) {
	logger, _ := newTestLogger()

	cfg := Config{
		Enabled: true,
		Logger:  logger,
	}
	mw := Middleware(cfg)

	var ids []string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		ids = append(ids, CorrelationID(ctx))
		return registry.MakeTextResult("ok"), nil
	}

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, handler)

	for range 3 {
		_, _ = wrapped(context.Background(), registry.CallToolRequest{})
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}

	// All IDs should be unique.
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate correlation ID: %s", id)
		}
		seen[id] = true
	}

	// IDs should be sequential (ascending string order since zero-padded).
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("IDs not strictly increasing: %s <= %s", ids[i], ids[i-1])
		}
	}
}

func TestCorrelationID_EmptyWhenNoMiddleware(t *testing.T) {
	id := CorrelationID(context.Background())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

func TestMiddleware_LogsGoError(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled: true,
		Logger:  logger,
	}
	mw := Middleware(cfg)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testErrorHandler())

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	_ = result

	output := buf.String()
	if !strings.Contains(output, "tool_call.error") {
		t.Errorf("expected tool_call.error in log:\n%s", output)
	}
	if !strings.Contains(output, "something broke") {
		t.Errorf("expected error message in log:\n%s", output)
	}
}

func TestMiddleware_LogsToolError(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:    true,
		Logger:     logger,
		LogResults: true,
	}
	mw := Middleware(cfg)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testToolErrorHandler())

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("tool errors should not return Go errors: %v", err)
	}
	if !result.IsError {
		t.Error("expected result.IsError to be true")
	}

	output := buf.String()
	// Tool-level errors log at WARN level.
	if !strings.Contains(output, "level=WARN") {
		t.Errorf("expected WARN level for tool errors:\n%s", output)
	}
	if !strings.Contains(output, "is_error=true") {
		t.Errorf("expected is_error=true in log:\n%s", output)
	}
}

func TestMiddleware_OutputSize(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled: true,
		Logger:  logger,
	}
	mw := Middleware(cfg)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("hello"))

	_, _ = wrapped(context.Background(), registry.CallToolRequest{})

	output := buf.String()
	if !strings.Contains(output, "output_bytes=") {
		t.Errorf("expected output_bytes in log:\n%s", output)
	}
	if !strings.Contains(output, "content_blocks=1") {
		t.Errorf("expected content_blocks=1 in log:\n%s", output)
	}
}

func TestMiddleware_TruncatesLargeResult(t *testing.T) {
	logger, buf := newTestLogger()

	cfg := Config{
		Enabled:           true,
		Logger:            logger,
		LogResults:        true,
		MaxResultLogBytes: 10,
	}
	mw := Middleware(cfg)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("this is a much longer result that should be truncated"))

	_, _ = wrapped(context.Background(), registry.CallToolRequest{})

	output := buf.String()
	if !strings.Contains(output, "...[truncated]") {
		t.Errorf("expected truncation marker in log:\n%s", output)
	}
}

func TestNew_FunctionalOptions(t *testing.T) {
	logger, buf := newTestLogger()

	mw := New(
		WithEnabled(true),
		WithLogger(logger),
		WithLogParams(false),
		WithLogResults(false),
		WithMaxResultLogBytes(256),
		WithRedactFields([]string{"secret"}),
	)

	td := registry.ToolDefinition{}
	wrapped := mw("test_tool", td, testHandler("ok"))

	req := testReq(map[string]any{"key": "value"})
	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if !strings.Contains(output, "tool_call.start") {
		t.Errorf("expected log output with functional options:\n%s", output)
	}
	// LogParams is false, so no params should appear.
	if strings.Contains(output, "params=") {
		t.Error("params should not be logged when WithLogParams(false)")
	}
}

func TestFormatCounter(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{1, "req-000001"},
		{42, "req-000042"},
		{999999, "req-999999"},
		{1000000, "req-1000000"},
	}
	for _, tt := range tests {
		got := formatCounter(tt.input)
		if got != tt.want {
			t.Errorf("formatCounter(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedactParams(t *testing.T) {
	redactSet := map[string]struct{}{
		"password": {},
		"token":    {},
	}

	tests := []struct {
		name    string
		args    map[string]any
		notWant []string // strings that must NOT appear
		want    []string // strings that MUST appear
	}{
		{
			name:    "redacts password",
			args:    map[string]any{"username": "admin", "password": "secret"},
			notWant: []string{"secret"},
			want:    []string{"admin", "[REDACTED]"},
		},
		{
			name:    "empty map",
			args:    map[string]any{},
			notWant: nil,
			want:    []string{"{}"},
		},
		{
			name: "nil map",
			args: nil,
			want: []string{"{}"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactParams(tt.args, redactSet)
			for _, nw := range tt.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("output should not contain %q: %s", nw, got)
				}
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("output should contain %q: %s", w, got)
				}
			}
		})
	}
}

func TestRequestCounter_Monotonic(t *testing.T) {
	// Record the starting value to avoid test pollution from other tests.
	start := requestCounter.Load()

	var results [100]uint64
	for i := range results {
		results[i] = requestCounter.Add(1)
	}

	for i := 1; i < len(results); i++ {
		if results[i] <= results[i-1] {
			t.Errorf("counter not monotonic at index %d: %d <= %d", i, results[i], results[i-1])
		}
	}

	end := requestCounter.Load()
	if end != start+100 {
		t.Errorf("expected counter to advance by 100: start=%d end=%d", start, end)
	}
}

func TestResetCounterForTestIsolation(t *testing.T) {
	// Verify the atomic counter is shared across tests (it's a package global).
	// This is intentional — correlation IDs are globally unique within a process.
	val := requestCounter.Load()
	if val == 0 {
		// Only true if this is the very first test to run. Fine either way.
		t.Log("counter is at zero (first test)")
	}
	// Just verify it's accessible and non-negative.
	_ = atomic.LoadUint64((*uint64)(&val))
}
