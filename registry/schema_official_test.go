//go:build official_sdk

package registry

func objectInputSchema() ToolInputSchema {
	return map[string]any{"type": "object"}
}

func emptyOutputSchema() *ToolOutputSchema {
	var schema ToolOutputSchema = map[string]any{"type": "object"}
	return &schema
}
