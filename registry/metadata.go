package registry

const anthropicMaxResultSizeMetaKey = "anthropic/maxResultSizeChars"

// ApplyToolMetadata applies annotations plus any descriptor-level metadata such
// as output schema, max-result hints, and defer-loading flags.
func ApplyToolMetadata(td ToolDefinition, prefix string, forceDeferred bool) ToolDefinition {
	td = ApplyMCPAnnotations(td, prefix)

	if td.OutputSchema != nil {
		td.Tool.OutputSchema = *td.OutputSchema
	}
	if td.MaxResultChars > 0 {
		SetToolMetaField(&td.Tool, anthropicMaxResultSizeMetaKey, td.MaxResultChars)
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
