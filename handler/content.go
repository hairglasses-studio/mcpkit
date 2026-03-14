package handler

import (
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

// FilterByAudience filters content blocks by the annotations.audience field.
// Only content with a matching audience (or no audience set) is retained.
func FilterByAudience(result *mcp.CallToolResult, audience []string) *mcp.CallToolResult {
	if result == nil || len(result.Content) == 0 {
		return result
	}

	audienceSet := make(map[string]bool, len(audience))
	for _, a := range audience {
		audienceSet[a] = true
	}

	var filtered []mcp.Content
	for _, c := range result.Content {
		ann := contentAnnotations(c)
		if ann == nil || len(ann.Audience) == 0 {
			filtered = append(filtered, c)
			continue
		}
		for _, a := range ann.Audience {
			if audienceSet[string(a)] {
				filtered = append(filtered, c)
				break
			}
		}
	}

	result.Content = filtered
	return result
}

// SortByPriority sorts content blocks by annotations.priority (highest first).
// Content without priority is treated as priority 0.
func SortByPriority(result *mcp.CallToolResult) *mcp.CallToolResult {
	if result == nil || len(result.Content) <= 1 {
		return result
	}

	sort.SliceStable(result.Content, func(i, j int) bool {
		pi := contentPriority(result.Content[i])
		pj := contentPriority(result.Content[j])
		return pi > pj
	})

	return result
}

// EstimateTokens provides a rough token count estimate using the chars/4 heuristic.
func EstimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// contentAnnotations extracts annotations from a content block.
func contentAnnotations(c mcp.Content) *mcp.Annotations {
	switch v := c.(type) {
	case mcp.TextContent:
		return v.Annotations
	case mcp.ImageContent:
		return v.Annotations
	case mcp.AudioContent:
		return v.Annotations
	default:
		return nil
	}
}

// contentPriority extracts the priority from a content block's annotations.
func contentPriority(c mcp.Content) float64 {
	ann := contentAnnotations(c)
	if ann == nil || ann.Priority == nil {
		return 0
	}
	return *ann.Priority
}
