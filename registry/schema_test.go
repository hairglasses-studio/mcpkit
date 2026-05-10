//go:build !official_sdk

package registry

func objectInputSchema() ToolInputSchema {
	return ToolInputSchema{Type: "object"}
}

func emptyOutputSchema() *ToolOutputSchema {
	return &ToolOutputSchema{}
}
