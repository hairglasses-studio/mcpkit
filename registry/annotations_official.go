//go:build official_sdk

package registry

import "strings"

// InferIsWrite determines if a tool modifies state based on its name suffix.
func InferIsWrite(name string) bool {
	writeSuffixes := []string{
		"_create", "_delete", "_remove", "_reset", "_send", "_post",
		"_update", "_set", "_add", "_apply", "_import", "_publish",
		"_start", "_stop", "_restart", "_trigger", "_execute", "_run",
		"_record", "_assign", "_unassign", "_move", "_copy", "_rename",
		"_enable", "_disable", "_clear", "_flush", "_purge",
		"_archive", "_restore", "_sync", "_push", "_deploy", "_install",
		"_uninstall", "_register", "_deregister", "_subscribe", "_unsubscribe",
		"_approve", "_reject", "_resolve", "_close", "_reopen",
	}
	nameLower := strings.ToLower(name)
	for _, suffix := range writeSuffixes {
		if strings.HasSuffix(nameLower, suffix) {
			return true
		}
	}
	return false
}

// ApplyMCPAnnotations applies MCP 2025 annotations based on tool metadata.
// The prefix is stripped from tool names when generating human-readable
// titles. Mirrors annotations.go's (mcp-go) structure and logic exactly, so
// the two builds agree on every declared hint's value AND declared-ness for
// the same tool — see this file's Annotation*Hint doc comments for why that
// parity matters. Two behaviors this brings into line with mcp-go, both
// previously gaps here:
//   - DestructiveHint is now unconditionally set (false for read-only tools
//     and non-matching write tools, true only for suffix-matched or
//     override-forced ones) instead of staying nil except for suffix
//     matches — AnnotationDestructiveHint's declared-ness now agrees across
//     tags for the same tool.
//   - SetToolTitle(&td.Tool, title) is now called, setting the top-level
//     Tool.Title field (previously only Tool.Annotations.Title was set,
//     which is the legacy/deprecated location — mcp-go already set both).
func ApplyMCPAnnotations(td ToolDefinition, prefix string) ToolDefinition {
	if td.ReadOnlyOverride != nil {
		td.IsWrite = !*td.ReadOnlyOverride
	}
	title := toolNameToTitle(td.Tool.Name, prefix)
	SetToolTitle(&td.Tool, title)

	destructive := false
	if td.IsWrite {
		nameLower := strings.ToLower(td.Tool.Name)
		for _, suffix := range []string{"_delete", "_remove", "_reset", "_purge", "_clear", "_flush", "_destroy", "_restart", "_expire"} {
			if strings.HasSuffix(nameLower, suffix) {
				destructive = true
				break
			}
		}
	}
	if td.DestructiveOverride != nil {
		destructive = *td.DestructiveOverride
	}

	idempotent := !td.IsWrite
	if td.IsWrite {
		nameLower := strings.ToLower(td.Tool.Name)
		for _, suffix := range []string{"_set", "_update", "_sync", "_enable", "_disable", "_assign", "_restart"} {
			if strings.HasSuffix(nameLower, suffix) {
				idempotent = true
				break
			}
		}
	}
	if td.IdempotentOverride != nil {
		idempotent = *td.IdempotentOverride
	}

	openWorld := true

	td.Tool.Annotations = &ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    !td.IsWrite,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   &openWorld,
	}

	return td
}

// AnnotationReadOnlyHint returns t.Annotations.ReadOnlyHint, plus whether it
// was declared. The official SDK's ReadOnlyHint is a plain bool (mcp-go's is
// *bool — see annotations.go's counterpart in this file), so there is no
// per-field way to distinguish "unset" from "explicitly false"; "declared"
// here means Annotations itself is non-nil (an Annotations struct was
// constructed at all, e.g. via ApplyMCPAnnotations, which always sets
// ReadOnlyHint alongside it).
func AnnotationReadOnlyHint(t Tool) (bool, bool) {
	if t.Annotations == nil {
		return false, false
	}
	return t.Annotations.ReadOnlyHint, true
}

// AnnotationDestructiveHint returns t.Annotations.DestructiveHint
// dereferenced, plus whether it was declared. DestructiveHint is *bool on
// this build (unlike ReadOnlyHint/IdempotentHint), so declared-ness is a
// real per-field signal here: Annotations non-nil AND the pointer non-nil.
// As of round 7 (2026-08-08), ApplyMCPAnnotations in this file always sets
// this pointer (matching mcp-go's own always-set behavior — previously it
// only did so for suffix-matched write tools, leaving it undeclared
// everywhere else and disagreeing with mcp-go's declared-ness for the same
// tool). A Tool built without going through ApplyMCPAnnotations can still
// legitimately leave it nil, hence this accessor's own nil-safety remains.
func AnnotationDestructiveHint(t Tool) (bool, bool) {
	if t.Annotations == nil || t.Annotations.DestructiveHint == nil {
		return false, false
	}
	return *t.Annotations.DestructiveHint, true
}

// AnnotationIdempotentHint returns t.Annotations.IdempotentHint, plus
// whether it was declared. Plain bool on this build (see
// AnnotationReadOnlyHint's doc comment for why "declared" means Annotations
// != nil rather than a per-field pointer check).
func AnnotationIdempotentHint(t Tool) (bool, bool) {
	if t.Annotations == nil {
		return false, false
	}
	return t.Annotations.IdempotentHint, true
}

// AnnotationOpenWorldHint returns t.Annotations.OpenWorldHint dereferenced,
// plus whether it was declared. *bool on this build (like DestructiveHint),
// so declared-ness is a real per-field signal.
func AnnotationOpenWorldHint(t Tool) (bool, bool) {
	if t.Annotations == nil || t.Annotations.OpenWorldHint == nil {
		return false, false
	}
	return *t.Annotations.OpenWorldHint, true
}

// toolNameToTitle converts a tool name like "myapp_gmail_send" to "Gmail Send".
func toolNameToTitle(name, prefix string) string {
	if prefix != "" {
		name = strings.TrimPrefix(name, prefix)
	}
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}
