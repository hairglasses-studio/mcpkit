package registry

const (
	anthropicMaxResultSizeMetaKey       = "anthropic/maxResultSizeChars"
	anthropicAlwaysLoadMetaKey          = "anthropic/alwaysLoad"
	anthropicRequiresUserInteractionKey = "anthropic/requiresUserInteraction"
)

// ApplyToolMetadata applies annotations plus any descriptor-level metadata such
// as output schema, max-result hints, always-load flags, defer-loading flags,
// and the requires-user-interaction consent flag.
func ApplyToolMetadata(td ToolDefinition, prefix string, forceDeferred bool) ToolDefinition {
	td = ApplyMCPAnnotations(td, prefix)

	if td.OutputSchema != nil {
		td.Tool.OutputSchema = *td.OutputSchema
	}
	if td.MaxResultChars > 0 {
		SetToolMetaField(&td.Tool, anthropicMaxResultSizeMetaKey, td.MaxResultChars)
	}
	if td.AlwaysLoad {
		SetToolMetaField(&td.Tool, anthropicAlwaysLoadMetaKey, true)
	}
	// Every write tool gets a real Claude Code permission prompt
	// (_meta["anthropic/requiresUserInteraction"] = true, supported since
	// Claude Code >=2.1.199) regardless of allowlist/permission mode, unless
	// the tool definition explicitly opts out via SkipRequiresUserInteraction.
	// This complements, not replaces, any repo-side write-gate ceremony.
	if td.IsWrite && !td.SkipRequiresUserInteraction {
		SetToolMetaField(&td.Tool, anthropicRequiresUserInteractionKey, true)
	}

	SetToolDeferLoading(&td.Tool, forceDeferred || td.DeferLoading)
	return td
}

func effectiveMaxResponseSize(td ToolDefinition, defaultSize int) int {
	if td.MaxResultChars > 0 && (defaultSize <= 0 || td.MaxResultChars < defaultSize) {
		return td.MaxResultChars
	}
	return defaultSize
}
