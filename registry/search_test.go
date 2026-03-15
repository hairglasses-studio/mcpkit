//go:build !official_sdk

package registry

import (
	"context"
	"testing"
)

func makeSearchTool(name, description, category string, tags []string) ToolDefinition {
	return ToolDefinition{
		Tool:     Tool{Name: name, Description: description},
		Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category: category,
		Tags:     tags,
	}
}

func TestSearchTools_EmptyQuery(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name:  "mod",
		tools: []ToolDefinition{makeSearchTool("my_tool", "A useful tool", "cat", nil)},
	})

	results := r.SearchTools("")
	if results != nil {
		t.Errorf("empty query should return nil, got %v", results)
	}
}

func TestSearchTools_WhitespaceQuery(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name:  "mod",
		tools: []ToolDefinition{makeSearchTool("my_tool", "A useful tool", "cat", nil)},
	})

	results := r.SearchTools("   ")
	if results != nil {
		t.Errorf("whitespace query should return nil, got %v", results)
	}
}

func TestSearchTools_NoMatches(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("my_tool", "A useful tool", "cat", nil),
		},
	})

	results := r.SearchTools("zzzzzzzzzzz")
	if len(results) != 0 {
		t.Errorf("expected no matches for nonsense query, got %d", len(results))
	}
}

func TestSearchTools_ExactNameMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("discord_send_message", "Send a Discord message", "discord", nil),
			makeSearchTool("slack_list_channels", "List Slack channels", "slack", nil),
		},
	})

	results := r.SearchTools("discord_send_message")
	if len(results) == 0 {
		t.Fatal("expected at least one result for exact name match")
	}
	if results[0].Tool.Tool.Name != "discord_send_message" {
		t.Errorf("top result = %q, want discord_send_message", results[0].Tool.Tool.Name)
	}
	if results[0].MatchType != "name" {
		t.Errorf("match type = %q, want name", results[0].MatchType)
	}
}

func TestSearchTools_PrefixMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("discord_send_message", "Send a Discord message", "discord", nil),
			makeSearchTool("discord_list_channels", "List Discord channels", "discord", nil),
			makeSearchTool("slack_list_channels", "List Slack channels", "slack", nil),
		},
	})

	results := r.SearchTools("discord")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'discord', got %d", len(results))
	}
	for _, r := range results {
		if r.MatchType != "name" && r.MatchType != "category" {
			t.Errorf("unexpected match type %q for discord prefix search", r.MatchType)
		}
	}
}

func TestSearchTools_DescriptionMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("tool_alpha", "Retrieves financial reports from the database", "finance", nil),
			makeSearchTool("tool_beta", "Sends emails to recipients", "email", nil),
		},
	})

	results := r.SearchTools("financial")
	if len(results) == 0 {
		t.Fatal("expected at least one result matching description term 'financial'")
	}
	found := false
	for _, r := range results {
		if r.Tool.Tool.Name == "tool_alpha" {
			found = true
		}
	}
	if !found {
		t.Error("expected tool_alpha to match description query 'financial'")
	}
}

func TestSearchTools_FuzzyMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("github_create_issue", "Create a GitHub issue", "github", nil),
			makeSearchTool("slack_send_message", "Send a Slack message", "slack", nil),
		},
	})

	// "githb" is one edit away from "github"
	results := r.SearchTools("githb")
	if len(results) == 0 {
		t.Fatal("expected fuzzy match for 'githb' to match 'github'")
	}
	if results[0].Tool.Tool.Name != "github_create_issue" {
		t.Errorf("top fuzzy result = %q, want github_create_issue", results[0].Tool.Tool.Name)
	}
}

func TestSearchTools_TagMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("tool_alpha", "Does something", "cat", []string{"messaging", "realtime"}),
			makeSearchTool("tool_beta", "Does other thing", "cat", []string{"storage", "database"}),
		},
	})

	results := r.SearchTools("messaging")
	if len(results) == 0 {
		t.Fatal("expected match on tag 'messaging'")
	}
	if results[0].Tool.Tool.Name != "tool_alpha" {
		t.Errorf("expected tool_alpha to be top result, got %q", results[0].Tool.Tool.Name)
	}
	if results[0].MatchType != "tag" {
		t.Errorf("match type = %q, want tag", results[0].MatchType)
	}
}

func TestSearchTools_CategoryMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("tool_alpha", "Does something", "payments", nil),
			makeSearchTool("tool_beta", "Does another thing", "payments", nil),
			makeSearchTool("tool_gamma", "Does a third thing", "analytics", nil),
		},
	})

	results := r.SearchTools("payments")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results for category 'payments', got %d", len(results))
	}
	for _, r := range results {
		if r.Tool.Category != "payments" {
			t.Errorf("expected only payments category tools, got %q", r.Tool.Category)
		}
	}
}

func TestSearchTools_MultiWordQuery(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeSearchTool("github_list_issues", "List GitHub issues", "github", nil),
			makeSearchTool("github_create_issue", "Create a GitHub issue", "github", nil),
			makeSearchTool("slack_list_channels", "List Slack channels", "slack", nil),
		},
	})

	// Both words must match — "github" and "list" should match github_list_issues
	results := r.SearchTools("github list")
	if len(results) == 0 {
		t.Fatal("expected results for multi-word query 'github list'")
	}
	// github_list_issues should be in results
	found := false
	for _, r := range results {
		if r.Tool.Tool.Name == "github_list_issues" {
			found = true
		}
	}
	if !found {
		t.Error("expected github_list_issues to match 'github list'")
	}
}

func TestSearchTools_IDFScoring(t *testing.T) {
	// Tools with a term shared by many tools should score lower for that term
	// than tools with a rare term. We test that the rare-term tool ranks higher.
	r := NewToolRegistry()

	// "tool" appears in all names — should reduce its IDF weight
	tools := []ToolDefinition{
		makeSearchTool("tool_alpha", "Common operation description", "cat", nil),
		makeSearchTool("tool_beta", "Common operation description", "cat", nil),
		makeSearchTool("tool_gamma", "Common operation description", "cat", nil),
		makeSearchTool("tool_delta", "Common operation description", "cat", nil),
		// This one has a unique term "xyzzy" in its description
		makeSearchTool("tool_epsilon", "Xyzzy unique special operation", "cat", nil),
	}
	r.RegisterModule(&testModule{name: "mod", tools: tools})

	// Search for a term that only matches one tool
	results := r.SearchTools("xyzzy")
	if len(results) == 0 {
		t.Fatal("expected at least one result for rare term 'xyzzy'")
	}
	if results[0].Tool.Tool.Name != "tool_epsilon" {
		t.Errorf("top result = %q, want tool_epsilon", results[0].Tool.Tool.Name)
	}
}

func TestSearchTools_ResultsSortedByScore(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			// Exact name match — should score higher
			makeSearchTool("send_email", "Send an email", "email", nil),
			// Description-only match
			makeSearchTool("compose_message", "Compose and send email to recipients", "messaging", nil),
		},
	})

	results := r.SearchTools("send")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// Verify results are ordered by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: result[%d].Score=%d > result[%d].Score=%d",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSearchTools_EmptyRegistry(t *testing.T) {
	r := NewToolRegistry()

	results := r.SearchTools("anything")
	if len(results) != 0 {
		t.Errorf("empty registry should return no results, got %d", len(results))
	}
}

func TestSearchTools_RuntimeGroupMatch(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name: "mod",
		tools: []ToolDefinition{
			{
				Tool:         Tool{Name: "tool_alpha", Description: "Does something"},
				Handler:      func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
				Category:     "cat",
				RuntimeGroup: "payment-services",
			},
			{
				Tool:         Tool{Name: "tool_beta", Description: "Does other things"},
				Handler:      func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
				Category:     "cat",
				RuntimeGroup: "analytics",
			},
		},
	})

	results := r.SearchTools("payment")
	if len(results) == 0 {
		t.Fatal("expected match on runtime_group 'payment-services'")
	}
	if results[0].Tool.Tool.Name != "tool_alpha" {
		t.Errorf("expected tool_alpha to match, got %q", results[0].Tool.Tool.Name)
	}
}
