package finops

import (
	"encoding/json"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultEstimate returns an approximate token count using the 4-chars-per-token heuristic.
func DefaultEstimate(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4 // ceiling division
}

// EstimateFromRequest estimates input tokens from a CallToolRequest.
func EstimateFromRequest(req registry.CallToolRequest, estimate func(string) int) int {
	args := registry.ExtractArguments(req)
	if args == nil {
		return 0
	}
	data, err := json.Marshal(args)
	if err != nil {
		return 0
	}
	return estimate(string(data))
}

// EstimateFromResult estimates output tokens from a CallToolResult.
func EstimateFromResult(result *registry.CallToolResult, estimate func(string) int) int {
	if result == nil || len(result.Content) == 0 {
		return 0
	}
	total := 0
	for _, c := range result.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			total += estimate(text)
		}
	}
	return total
}
