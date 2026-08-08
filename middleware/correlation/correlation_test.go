package correlation

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddleware_InjectsRandomID(t *testing.T) {
	t.Parallel()

	var captured string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		captured = FromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := Middleware(Config{})("test", registry.ToolDefinition{}, handler)
	_, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(captured) != 16 {
		t.Errorf("correlation ID length = %d, want 16 (hex of 8 bytes); got=%q", len(captured), captured)
	}
}

func TestMiddleware_UniquePerCall(t *testing.T) {
	t.Parallel()

	seen := make([]string, 0, 100)
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		seen = append(seen, FromContext(ctx))
		return registry.MakeTextResult("ok"), nil
	}
	wrapped := Middleware(Config{})("test", registry.ToolDefinition{}, handler)

	for i := 0; i < 100; i++ {
		if _, err := wrapped(context.Background(), registry.CallToolRequest{}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	uniq := make(map[string]struct{}, len(seen))
	for _, id := range seen {
		uniq[id] = struct{}{}
	}
	if len(uniq) != len(seen) {
		t.Errorf("expected 100 unique IDs, got %d unique of %d total", len(uniq), len(seen))
	}
}

func TestMiddleware_GenerateIDOverride(t *testing.T) {
	t.Parallel()

	var captured string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		captured = FromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := Middleware(Config{
		GenerateID: func(_ context.Context, _ registry.CallToolRequest) string {
			return "from-upstream-lb"
		},
	})("test", registry.ToolDefinition{}, handler)

	if _, err := wrapped(context.Background(), registry.CallToolRequest{}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if captured != "from-upstream-lb" {
		t.Errorf("GenerateID override not applied; got %q", captured)
	}
}

func TestMiddleware_LoggerEnrichment(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		LoggerFromContext(ctx).Info("inside handler", "foo", "bar")
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := Middleware(Config{
		Logger:      logger,
		HeaderField: "request_id",
		GenerateID:  func(_ context.Context, _ registry.CallToolRequest) string { return "fixed-id" },
	})("my_tool", registry.ToolDefinition{}, handler)

	if _, err := wrapped(context.Background(), registry.CallToolRequest{}); err != nil {
		t.Fatalf("call: %v", err)
	}

	// The captured log line must include both the correlation ID under
	// the configured field name and the auto-injected tool name.
	out := buf.String()
	if !strings.Contains(out, `"request_id":"fixed-id"`) {
		t.Errorf("log missing request_id field; got=%s", out)
	}
	if !strings.Contains(out, `"tool":"my_tool"`) {
		t.Errorf("log missing tool field; got=%s", out)
	}
	if !strings.Contains(out, `"foo":"bar"`) {
		t.Errorf("log missing handler-supplied field; got=%s", out)
	}

	// Confirm it parses as valid JSON (regression guard against malformed slog output).
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Errorf("log line not valid JSON: %v; raw=%s", err, out)
	}
}

func TestFromContext_EmptyWhenUnwired(t *testing.T) {
	t.Parallel()

	if got := FromContext(context.Background()); got != "" {
		t.Errorf("FromContext on bare ctx = %q, want empty", got)
	}
}

func TestLoggerFromContext_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	// Unconfigured context returns slog.Default() — same instance.
	if LoggerFromContext(context.Background()) != slog.Default() {
		t.Error("LoggerFromContext on bare ctx should return slog.Default()")
	}
}
