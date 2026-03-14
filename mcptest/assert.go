package mcptest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// AssertToolResult checks that the result contains the expected text content.
func AssertToolResult(t testing.TB, result *registry.CallToolResult, expected string) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	text := ExtractText(t, result)
	if text != expected {
		t.Errorf("result text = %q, want %q", text, expected)
	}
}

// AssertToolResultContains checks that the result text contains a substring.
func AssertToolResultContains(t testing.TB, result *registry.CallToolResult, substr string) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	text := ExtractText(t, result)
	if !strings.Contains(text, substr) {
		t.Errorf("result text %q does not contain %q", text, substr)
	}
}

// AssertError checks that the result is an error with the given code prefix.
func AssertError(t testing.TB, result *registry.CallToolResult, code string) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if !registry.IsResultError(result) {
		t.Fatal("expected error result, got success")
	}
	if code != "" {
		text := ExtractText(t, result)
		prefix := "[" + code + "]"
		if !strings.HasPrefix(text, prefix) {
			t.Errorf("error text %q does not start with %q", text, prefix)
		}
	}
}

// AssertNotError checks that the result is not an error.
func AssertNotError(t testing.TB, result *registry.CallToolResult) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if registry.IsResultError(result) {
		text := ExtractText(t, result)
		t.Fatalf("expected success result, got error: %s", text)
	}
}

// AssertStructured unmarshals the structured content into the target and validates.
func AssertStructured(t testing.TB, result *registry.CallToolResult, target interface{}) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.StructuredContent == nil {
		t.Fatal("structured content is nil")
	}

	// Re-marshal and unmarshal to populate the target
	bytes, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("failed to marshal structured content: %v", err)
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		t.Fatalf("failed to unmarshal structured content: %v", err)
	}
}

// ExtractText extracts the text from the first TextContent in a result.
func ExtractText(t testing.TB, result *registry.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatalf("first content is not TextContent, got %T", result.Content[0])
	}
	return text
}
