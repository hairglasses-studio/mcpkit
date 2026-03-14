package handler

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func ptrFloat(f float64) *float64 { return &f }

func TestFilterByAudience_KeepsMatching(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Audience: []mcp.Role{"user"}},
				},
				Type: "text",
				Text: "user content",
			},
			mcp.TextContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant"}},
				},
				Type: "text",
				Text: "assistant content",
			},
		},
	}

	filtered := FilterByAudience(result, []string{"user"})
	if len(filtered.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(filtered.Content))
	}
	tc := filtered.Content[0].(mcp.TextContent)
	if tc.Text != "user content" {
		t.Errorf("text = %q, want %q", tc.Text, "user content")
	}
}

func TestFilterByAudience_KeepsNoAnnotation(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "no annotation"},
		},
	}

	filtered := FilterByAudience(result, []string{"user"})
	if len(filtered.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(filtered.Content))
	}
}

func TestFilterByAudience_NilResult(t *testing.T) {
	if FilterByAudience(nil, []string{"user"}) != nil {
		t.Error("should return nil for nil result")
	}
}

func TestSortByPriority(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Priority: ptrFloat(0.1)},
				},
				Type: "text",
				Text: "low",
			},
			mcp.TextContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Priority: ptrFloat(0.9)},
				},
				Type: "text",
				Text: "high",
			},
			mcp.TextContent{
				Type: "text",
				Text: "none",
			},
		},
	}

	sorted := SortByPriority(result)
	texts := make([]string, len(sorted.Content))
	for i, c := range sorted.Content {
		texts[i] = c.(mcp.TextContent).Text
	}
	if texts[0] != "high" {
		t.Errorf("first = %q, want high", texts[0])
	}
	if texts[2] != "none" {
		t.Errorf("last = %q, want none", texts[2])
	}
}

func TestSortByPriority_NilResult(t *testing.T) {
	if SortByPriority(nil) != nil {
		t.Error("should return nil for nil result")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hi", 1},
		{"hello world", 3},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
	}
	for _, tt := range tests {
		got := EstimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}
