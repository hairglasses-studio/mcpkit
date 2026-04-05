//go:build !official_sdk

package truncate

import (
	"context"
	"strings"
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

// testMultiContentHandler returns a handler producing multiple text content blocks.
func testMultiContentHandler(texts ...string) registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		content := make([]mcp.Content, len(texts))
		for i, t := range texts {
			content[i] = registry.MakeTextContent(t)
		}
		return &registry.CallToolResult{Content: content}, nil
	}
}

// testErrorHandler returns a handler that produces a tool-level error result.
func testErrorHandler(text string) registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult(text), nil
	}
}

// testReq builds a minimal CallToolRequest.
func testReq() registry.CallToolRequest {
	return registry.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "test_tool",
		},
	}
}

// extractAllText returns the concatenated text from all content blocks.
func extractAllText(result *registry.CallToolResult) string {
	var sb strings.Builder
	for _, block := range result.Content {
		if text, ok := registry.ExtractTextContent(block); ok {
			sb.WriteString(text)
		}
	}
	return sb.String()
}

func TestSmallResponsePassesThrough(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 4096, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	small := "hello world"
	wrapped := mw("test_tool", td, testHandler(small))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have exactly one content block (no guidance appended).
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if text != small {
		t.Errorf("expected %q, got %q", small, text)
	}
}

func TestLargeResponseTruncated(t *testing.T) {
	limit := 100
	mw := Middleware(Config{MaxBytes: limit, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	// Generate text larger than limit.
	large := strings.Repeat("x", 500)
	wrapped := mw("test_tool", td, testHandler(large))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have 2 blocks: truncated text + guidance message.
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}

	// First block should be truncated to exactly limit bytes.
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content in first block")
	}
	if len(text) != limit {
		t.Errorf("expected truncated text length %d, got %d", limit, len(text))
	}

	// Second block should be the guidance message.
	msg, ok := registry.ExtractTextContent(result.Content[1])
	if !ok {
		t.Fatal("expected text content in guidance block")
	}
	if msg != DefaultMessage {
		t.Errorf("expected guidance message %q, got %q", DefaultMessage, msg)
	}
}

func TestHardMaxEnforced(t *testing.T) {
	// MaxBytes exceeds HardMax — HardMax should win.
	mw := Middleware(Config{MaxBytes: 50000, HardMax: 200, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	large := strings.Repeat("a", 1000)
	wrapped := mw("test_tool", td, testHandler(large))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First block should be truncated to HardMax (200).
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if len(text) != 200 {
		t.Errorf("expected truncated to HardMax 200, got %d", len(text))
	}
}

func TestCustomMessage(t *testing.T) {
	custom := "Please narrow your search."
	mw := Middleware(Config{MaxBytes: 10, HardMax: 16384, Message: custom})
	td := registry.ToolDefinition{}

	large := strings.Repeat("z", 100)
	wrapped := mw("test_tool", td, testHandler(large))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Last block should be the custom message.
	last := result.Content[len(result.Content)-1]
	msg, ok := registry.ExtractTextContent(last)
	if !ok {
		t.Fatal("expected text content in guidance block")
	}
	if msg != custom {
		t.Errorf("expected custom message %q, got %q", custom, msg)
	}
}

func TestZeroConfigDefaults(t *testing.T) {
	// Using New() with no options should apply defaults.
	mw := New()
	td := registry.ToolDefinition{}

	// A response under DefaultMaxBytes should pass through.
	small := strings.Repeat("s", DefaultMaxBytes-1)
	wrapped := mw("test_tool", td, testHandler(small))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 block (no truncation), got %d", len(result.Content))
	}

	// A response over DefaultMaxBytes should be truncated.
	large := strings.Repeat("l", DefaultMaxBytes+500)
	wrapped = mw("test_tool", td, testHandler(large))

	result, err = wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 blocks (truncated + guidance), got %d", len(result.Content))
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	if len(text) != DefaultMaxBytes {
		t.Errorf("expected truncated to %d, got %d", DefaultMaxBytes, len(text))
	}
}

func TestErrorResponsePassesThrough(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 10, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	// Error response with text larger than limit should NOT be truncated.
	errorText := strings.Repeat("e", 100)
	wrapped := mw("test_tool", td, testErrorHandler(errorText))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError to be true")
	}

	// Should have exactly one content block (no truncation applied).
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block (error passthrough), got %d", len(result.Content))
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if text != errorText {
		t.Errorf("error text should be unchanged, got length %d", len(text))
	}
}

func TestNilResultPassesThrough(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 10, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	nilHandler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, nil
	}

	wrapped := mw("test_tool", td, nilHandler)
	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result to pass through")
	}
}

func TestMultipleContentBlocksTruncated(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 150, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	// Three blocks of 100 bytes each = 300 total, budget is 150.
	block := strings.Repeat("b", 100)
	wrapped := mw("test_tool", td, testMultiContentHandler(block, block, block))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First block: 100 bytes (fits, budget becomes 50).
	// Second block: 100 bytes, trimmed to 50 (budget becomes 0).
	// Third block: skipped (no budget).
	// Plus guidance message.
	if len(result.Content) != 3 {
		t.Fatalf("expected 3 content blocks (2 text + guidance), got %d", len(result.Content))
	}

	text0, _ := registry.ExtractTextContent(result.Content[0])
	if len(text0) != 100 {
		t.Errorf("first block: expected 100, got %d", len(text0))
	}

	text1, _ := registry.ExtractTextContent(result.Content[1])
	if len(text1) != 50 {
		t.Errorf("second block: expected 50, got %d", len(text1))
	}

	msg, _ := registry.ExtractTextContent(result.Content[2])
	if msg != DefaultMessage {
		t.Errorf("guidance message mismatch: %q", msg)
	}
}

func TestExactLimitNotTruncated(t *testing.T) {
	limit := 100
	mw := Middleware(Config{MaxBytes: limit, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	// Exactly at limit should NOT be truncated.
	exact := strings.Repeat("e", limit)
	wrapped := mw("test_tool", td, testHandler(exact))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 block (no truncation at exact limit), got %d", len(result.Content))
	}
}

func TestFunctionalOptionsApplied(t *testing.T) {
	custom := "narrow it down"
	mw := New(
		WithMaxBytes(50),
		WithHardMax(100),
		WithMessage(custom),
	)
	td := registry.ToolDefinition{}

	large := strings.Repeat("f", 200)
	wrapped := mw("test_tool", td, testHandler(large))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	if len(text) != 50 {
		t.Errorf("expected truncated to 50, got %d", len(text))
	}

	msg, _ := registry.ExtractTextContent(result.Content[len(result.Content)-1])
	if msg != custom {
		t.Errorf("expected custom message %q, got %q", custom, msg)
	}
}

func TestHandlerErrorPassesThrough(t *testing.T) {
	mw := Middleware(Config{MaxBytes: 10, HardMax: 16384, Message: DefaultMessage})
	td := registry.ToolDefinition{}

	goErrHandler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, context.DeadlineExceeded
	}

	wrapped := mw("test_tool", td, goErrHandler)
	result, err := wrapped(context.Background(), testReq())
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result on Go error")
	}
}
