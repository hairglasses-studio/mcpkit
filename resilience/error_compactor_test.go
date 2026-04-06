//go:build !official_sdk

package resilience

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func compactorMakeReq() registry.CallToolRequest {
	return mcp.CallToolRequest{}
}

func compactorErrorHandler(msg string) registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult(msg), nil
	}
}

func compactorSuccessHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

func TestErrorCompactorMiddleware_BelowThreshold(t *testing.T) {
	mw := ErrorCompactorMiddleware(ErrorCompactorConfig{Threshold: 3})
	td := registry.ToolDefinition{}
	handler := mw("test", td, compactorErrorHandler("fail"))

	// First two errors should pass through unchanged.
	for i := 0; i < 2; i++ {
		result, err := handler(context.Background(), compactorMakeReq())
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if result == nil {
			t.Fatalf("iteration %d: result is nil", i)
		}
		text, _ := registry.ExtractTextContent(result.Content[0])
		if text != "fail" {
			t.Errorf("iteration %d: text = %q, want %q", i, text, "fail")
		}
	}
}

func TestErrorCompactorMiddleware_AtThreshold(t *testing.T) {
	mw := ErrorCompactorMiddleware(ErrorCompactorConfig{Threshold: 3})
	td := registry.ToolDefinition{}
	handler := mw("test", td, compactorErrorHandler("fail"))

	// Call 3 times (at threshold).
	var result *registry.CallToolResult
	for i := 0; i < 3; i++ {
		result, _ = handler(context.Background(), compactorMakeReq())
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	if text == "fail" {
		t.Error("at threshold, error should be compacted")
	}
	if !registry.IsResultError(result) {
		t.Error("compacted result should still be an error")
	}
}

func TestErrorCompactorMiddleware_ResetOnSuccess(t *testing.T) {
	callCount := 0
	dynamicHandler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount++
		if callCount <= 2 {
			return registry.MakeErrorResult("fail"), nil
		}
		return registry.MakeTextResult("ok"), nil
	}

	mw := ErrorCompactorMiddleware(ErrorCompactorConfig{Threshold: 3})
	td := registry.ToolDefinition{}
	handler := mw("test", td, dynamicHandler)

	// Two errors, then success.
	handler(context.Background(), compactorMakeReq())
	handler(context.Background(), compactorMakeReq())
	handler(context.Background(), compactorMakeReq()) // success resets counter

	// Now two more errors should not trigger compaction (count restarted).
	callCount = 0 // reset to produce errors again
	for i := 0; i < 2; i++ {
		result, _ := handler(context.Background(), compactorMakeReq())
		text, _ := registry.ExtractTextContent(result.Content[0])
		if text != "fail" {
			t.Errorf("after reset, error %d should pass through unchanged, got %q", i, text)
		}
	}
}

func TestErrorCompactorMiddleware_SuccessPassesThrough(t *testing.T) {
	mw := ErrorCompactorMiddleware()
	td := registry.ToolDefinition{}
	handler := mw("test", td, compactorSuccessHandler)

	result, err := handler(context.Background(), compactorMakeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("success result should not be error")
	}
}

func TestErrorCompactorMiddlewareWithTracker(t *testing.T) {
	tracker := NewErrorCompactorTracker()
	mw := ErrorCompactorMiddlewareWithTracker(tracker, ErrorCompactorConfig{Threshold: 2})
	td := registry.ToolDefinition{}
	handler := mw("tracked_tool", td, compactorErrorHandler("oops"))

	handler(context.Background(), compactorMakeReq())
	if count := tracker.ConsecutiveErrors("tracked_tool"); count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	handler(context.Background(), compactorMakeReq())
	if count := tracker.ConsecutiveErrors("tracked_tool"); count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestDefaultErrorCompactorConfig(t *testing.T) {
	cfg := DefaultErrorCompactorConfig()
	if cfg.Threshold != 3 {
		t.Errorf("default threshold = %d, want 3", cfg.Threshold)
	}
}
