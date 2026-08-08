package registry

import "testing"

// TestAnnotationHints_ReadOnlyTool exercises the 4 Annotation*Hint accessors
// against a read-only tool run through ApplyMCPAnnotations, on both tags.
func TestAnnotationHints_ReadOnlyTool(t *testing.T) {
	td := ToolDefinition{Tool: Tool{Name: "test_get"}, IsWrite: false}
	annotated := ApplyMCPAnnotations(td, "")

	readOnly, ok := AnnotationReadOnlyHint(annotated.Tool)
	if !ok {
		t.Fatal("AnnotationReadOnlyHint: not declared")
	}
	if !readOnly {
		t.Error("AnnotationReadOnlyHint = false, want true for a read-only tool")
	}

	idempotent, ok := AnnotationIdempotentHint(annotated.Tool)
	if !ok {
		t.Fatal("AnnotationIdempotentHint: not declared")
	}
	if !idempotent {
		t.Error("AnnotationIdempotentHint = false, want true for a read-only tool")
	}
}

// TestAnnotationHints_DestructiveWriteTool exercises a write tool whose name
// matches ApplyMCPAnnotations' destructive-suffix heuristic (both
// implementations recognize "_delete").
func TestAnnotationHints_DestructiveWriteTool(t *testing.T) {
	td := ToolDefinition{Tool: Tool{Name: "test_thing_delete"}, IsWrite: true}
	annotated := ApplyMCPAnnotations(td, "")

	readOnly, ok := AnnotationReadOnlyHint(annotated.Tool)
	if !ok {
		t.Fatal("AnnotationReadOnlyHint: not declared")
	}
	if readOnly {
		t.Error("AnnotationReadOnlyHint = true, want false for a write tool")
	}

	destructive, ok := AnnotationDestructiveHint(annotated.Tool)
	if !ok {
		t.Fatal("AnnotationDestructiveHint: not declared for a _delete-suffixed write tool")
	}
	if !destructive {
		t.Error("AnnotationDestructiveHint = false, want true for a _delete-suffixed write tool")
	}
}

// TestAnnotationHints_OpenWorld confirms OpenWorldHint is always declared
// true after ApplyMCPAnnotations, on both tags.
func TestAnnotationHints_OpenWorld(t *testing.T) {
	td := ToolDefinition{Tool: Tool{Name: "test_get"}, IsWrite: false}
	annotated := ApplyMCPAnnotations(td, "")

	openWorld, ok := AnnotationOpenWorldHint(annotated.Tool)
	if !ok {
		t.Fatal("AnnotationOpenWorldHint: not declared")
	}
	if !openWorld {
		t.Error("AnnotationOpenWorldHint = false, want true")
	}
}

// TestAnnotationHints_Undeclared confirms all four accessors report
// undeclared (ok=false) on a bare Tool with no Annotations ever set.
func TestAnnotationHints_Undeclared(t *testing.T) {
	bare := Tool{Name: "untouched"}

	if _, ok := AnnotationReadOnlyHint(bare); ok {
		t.Error("AnnotationReadOnlyHint: expected undeclared on a bare Tool")
	}
	if _, ok := AnnotationDestructiveHint(bare); ok {
		t.Error("AnnotationDestructiveHint: expected undeclared on a bare Tool")
	}
	if _, ok := AnnotationIdempotentHint(bare); ok {
		t.Error("AnnotationIdempotentHint: expected undeclared on a bare Tool")
	}
	if _, ok := AnnotationOpenWorldHint(bare); ok {
		t.Error("AnnotationOpenWorldHint: expected undeclared on a bare Tool")
	}
}
