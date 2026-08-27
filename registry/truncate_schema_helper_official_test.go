//go:build official_sdk

package registry

// testOutputSchema builds the official-SDK shape of a declared output schema.
// See truncate_schema_helper_test.go for the mcp-go twin and why the split
// exists.
func testOutputSchema() *ToolOutputSchema {
	var schema ToolOutputSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rows":  map[string]any{"type": "array"},
			"total": map[string]any{"type": "integer"},
		},
		"required": []string{"rows", "total"},
	}
	return &schema
}
