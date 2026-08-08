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
// The prefix is stripped from tool names when generating human-readable titles.
func ApplyMCPAnnotations(td ToolDefinition, prefix string) ToolDefinition {
	td.Tool.Annotations = &ToolAnnotation{
		Title:        toolNameToTitle(td.Tool.Name, prefix),
		ReadOnlyHint: !td.IsWrite,
	}

	if td.IsWrite {
		nameLower := strings.ToLower(td.Tool.Name)
		for _, suffix := range []string{"_delete", "_remove", "_reset", "_purge", "_clear", "_flush", "_destroy", "_restart", "_expire"} {
			if strings.HasSuffix(nameLower, suffix) {
				destructive := true
				td.Tool.Annotations.DestructiveHint = &destructive
				break
			}
		}

		for _, suffix := range []string{"_set", "_update", "_sync", "_enable", "_disable", "_assign", "_restart"} {
			if strings.HasSuffix(nameLower, suffix) {
				td.Tool.Annotations.IdempotentHint = true
				break
			}
		}
	} else {
		td.Tool.Annotations.IdempotentHint = true
	}

	if td.DestructiveOverride != nil {
		td.Tool.Annotations.DestructiveHint = td.DestructiveOverride
	}
	if td.IdempotentOverride != nil {
		td.Tool.Annotations.IdempotentHint = *td.IdempotentOverride
	}

	openWorld := true
	td.Tool.Annotations.OpenWorldHint = &openWorld

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
