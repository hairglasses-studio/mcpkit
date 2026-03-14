//go:build official_sdk

package registry

import "testing"

// makeEmptyCallToolRequest returns a zero-value CallToolRequest for use in tests.
func makeEmptyCallToolRequest() CallToolRequest {
	return CallToolRequest{}
}

// assertReadOnlyHint checks that the tool's ReadOnlyHint annotation matches expected.
// In the official SDK, Annotations is a pointer and ReadOnlyHint is a plain bool.
func assertReadOnlyHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	if td.Tool.Annotations == nil {
		t.Errorf("Annotations is nil, want ReadOnlyHint = %v", expected)
		return
	}
	if td.Tool.Annotations.ReadOnlyHint != expected {
		t.Errorf("ReadOnlyHint = %v, want %v", td.Tool.Annotations.ReadOnlyHint, expected)
	}
}

// assertDestructiveHint checks that the tool's DestructiveHint annotation matches expected.
// In the official SDK, DestructiveHint is *bool and nil means false (not destructive).
func assertDestructiveHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	if td.Tool.Annotations == nil {
		if expected {
			t.Errorf("Annotations is nil, want DestructiveHint = %v", expected)
		}
		return
	}
	hint := td.Tool.Annotations.DestructiveHint
	// nil means false (not destructive) in the official SDK
	actual := hint != nil && *hint
	if actual != expected {
		t.Errorf("DestructiveHint = %v, want %v", actual, expected)
	}
}

// annotationTitle returns the Title from the tool's Annotations.
// In the official SDK, Annotations is a pointer.
func annotationTitle(td ToolDefinition) string {
	if td.Tool.Annotations == nil {
		return ""
	}
	return td.Tool.Annotations.Title
}
