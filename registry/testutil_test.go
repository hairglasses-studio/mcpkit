//go:build !official_sdk

package registry

import "testing"

// makeEmptyCallToolRequest returns a zero-value CallToolRequest for use in tests.
func makeEmptyCallToolRequest() CallToolRequest {
	return CallToolRequest{}
}

// assertReadOnlyHint checks that the tool's ReadOnlyHint annotation matches expected.
// In mcp-go, Annotations is a value struct and ReadOnlyHint is *bool.
func assertReadOnlyHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	hint := td.Tool.Annotations.ReadOnlyHint
	if hint == nil {
		t.Errorf("ReadOnlyHint is nil, want %v", expected)
		return
	}
	if *hint != expected {
		t.Errorf("ReadOnlyHint = %v, want %v", *hint, expected)
	}
}

// assertDestructiveHint checks that the tool's DestructiveHint annotation matches expected.
// In mcp-go, DestructiveHint is *bool.
func assertDestructiveHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	hint := td.Tool.Annotations.DestructiveHint
	if hint == nil {
		t.Errorf("DestructiveHint is nil, want %v", expected)
		return
	}
	if *hint != expected {
		t.Errorf("DestructiveHint = %v, want %v", *hint, expected)
	}
}

// annotationTitle returns the Title from the tool's Annotations.
// In mcp-go, Annotations is a value struct.
func annotationTitle(td ToolDefinition) string {
	return td.Tool.Annotations.Title
}
