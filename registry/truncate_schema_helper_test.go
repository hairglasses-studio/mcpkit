//go:build !official_sdk

package registry

// testOutputSchema builds the mcp-go shape of a declared output schema.
// Its official-SDK twin lives in truncate_schema_helper_official_test.go; the
// two SDKs type Tool.OutputSchema differently (a value struct vs `any`), so
// this is the one part of the over-cap fixture that cannot be written once.
func testOutputSchema() *ToolOutputSchema {
	schema := ToolOutputSchema{
		Type: "object",
		Properties: map[string]any{
			"rows":  map[string]any{"type": "array"},
			"total": map[string]any{"type": "integer"},
		},
		Required: []string{"rows", "total"},
	}
	return &schema
}
