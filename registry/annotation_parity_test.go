package registry

import "testing"

// TestApplyMCPAnnotations_Parity asserts a read-only tool and a
// non-destructive write tool get IDENTICAL declared-ness (not just value)
// for every hint, plus a non-empty display title, on both build tags. This
// is the parity round 6 couldn't write while the asymmetry existed: before
// this round's fix, official_sdk's ApplyMCPAnnotations left DestructiveHint
// undeclared (nil) for any tool that wasn't both IsWrite and suffix-matched
// -- so AnnotationDestructiveHint's declared-ness disagreed across tags for
// the READ-ONLY case and the "write tool with no matching suffix" case
// (e.g. a create/get-style write tool), even though mcp-go always declared
// it. Because this is one neutral test file asserting the same expected
// values under whichever tag is compiled in, both tags passing it together
// IS the parity proof -- there is nothing tag-specific left to diverge on.
func TestApplyMCPAnnotations_Parity(t *testing.T) {
	cases := []struct {
		name            string
		isWrite         bool
		wantReadOnly    bool
		wantDestructive bool
		wantIdempotent  bool
	}{
		// A read-only tool: no suffix in either heuristic list applies.
		{name: "test_thing_get", isWrite: false, wantReadOnly: true, wantDestructive: false, wantIdempotent: true},
		// A write tool whose name matches NEITHER the destructive-suffix nor
		// the idempotent-suffix list -- the exact case that was previously
		// undeclared for DestructiveHint on official_sdk.
		{name: "test_thing_create", isWrite: true, wantReadOnly: false, wantDestructive: false, wantIdempotent: false},
		// A write tool matching the destructive-suffix heuristic.
		{name: "test_thing_delete", isWrite: true, wantReadOnly: false, wantDestructive: true, wantIdempotent: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := ToolDefinition{Tool: Tool{Name: tc.name}, IsWrite: tc.isWrite}
			annotated := ApplyMCPAnnotations(td, "")

			if title := annotated.Tool.Title; title == "" {
				t.Error("Tool.Title is empty, want a non-empty display title on both tags")
			}

			ro, declared := AnnotationReadOnlyHint(annotated.Tool)
			if !declared {
				t.Fatal("AnnotationReadOnlyHint: not declared, want declared on both tags")
			}
			if ro != tc.wantReadOnly {
				t.Errorf("AnnotationReadOnlyHint = %v, want %v", ro, tc.wantReadOnly)
			}

			dh, declared := AnnotationDestructiveHint(annotated.Tool)
			if !declared {
				t.Fatal("AnnotationDestructiveHint: not declared, want declared on both tags (the round 7 fix)")
			}
			if dh != tc.wantDestructive {
				t.Errorf("AnnotationDestructiveHint = %v, want %v", dh, tc.wantDestructive)
			}

			ih, declared := AnnotationIdempotentHint(annotated.Tool)
			if !declared {
				t.Fatal("AnnotationIdempotentHint: not declared, want declared on both tags")
			}
			if ih != tc.wantIdempotent {
				t.Errorf("AnnotationIdempotentHint = %v, want %v", ih, tc.wantIdempotent)
			}

			ow, declared := AnnotationOpenWorldHint(annotated.Tool)
			if !declared {
				t.Fatal("AnnotationOpenWorldHint: not declared, want declared on both tags")
			}
			if !ow {
				t.Error("AnnotationOpenWorldHint = false, want true")
			}
		})
	}
}
