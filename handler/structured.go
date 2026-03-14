package handler

import (
	"encoding/json"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// StructuredResult creates a tool result with both structured content and
// a backwards-compatible text representation. This implements the MCP 2025-11-25
// structured output pattern — servers return structuredContent for typed parsing
// and content with serialized JSON for older clients.
func StructuredResult(data any) *registry.CallToolResult {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to marshal structured result: %w", err))
	}

	return registry.MakeStructuredResult(registry.MakeTextContent(string(bytes)), data)
}

// ResponseFormat represents the level of detail in a tool response.
// Offering this as a parameter allows agents to choose between detailed
// (first call, understanding context) and concise (subsequent calls,
// saving tokens) responses — a pattern that reduces token usage by ~65%.
type ResponseFormat string

const (
	// FormatDetailed returns full information with all fields.
	FormatDetailed ResponseFormat = "detailed"

	// FormatConcise returns minimal information — IDs, names, status only.
	FormatConcise ResponseFormat = "concise"
)

// GetResponseFormat extracts the response_format parameter from the request,
// defaulting to FormatDetailed if not specified.
func GetResponseFormat(req registry.CallToolRequest) ResponseFormat {
	format := GetStringParam(req, "response_format")
	switch ResponseFormat(format) {
	case FormatConcise:
		return FormatConcise
	case FormatDetailed:
		return FormatDetailed
	default:
		return FormatDetailed
	}
}

// ResponseFormatSchema returns the JSON Schema property definition for a
// response_format parameter. Add this to your tool's inputSchema properties.
func ResponseFormatSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"enum":        []string{"detailed", "concise"},
		"default":     "detailed",
		"description": "Response detail level. Use 'concise' for subsequent calls to save tokens, 'detailed' for full information.",
	}
}
