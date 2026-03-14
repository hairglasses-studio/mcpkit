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
		Title:    toolNameToTitle(td.Tool.Name, prefix),
		ReadOnlyHint: !td.IsWrite,
	}

	if td.IsWrite {
		nameLower := strings.ToLower(td.Tool.Name)
		for _, suffix := range []string{"_delete", "_remove", "_reset", "_purge", "_clear", "_flush", "_destroy"} {
			if strings.HasSuffix(nameLower, suffix) {
				destructive := true
				td.Tool.Annotations.DestructiveHint = &destructive
				break
			}
		}

		for _, suffix := range []string{"_set", "_update", "_sync", "_enable", "_disable", "_assign"} {
			if strings.HasSuffix(nameLower, suffix) {
				td.Tool.Annotations.IdempotentHint = true
				break
			}
		}
	} else {
		td.Tool.Annotations.IdempotentHint = true
	}

	openWorld := true
	td.Tool.Annotations.OpenWorldHint = &openWorld

	return td
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
