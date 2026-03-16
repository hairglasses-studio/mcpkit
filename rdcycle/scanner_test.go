package rdcycle

import (
	"context"
	"testing"
)

func TestScannerNoGH(t *testing.T) {
	// Scanner should gracefully return nil when gh is not available or repos empty.
	s := &Scanner{Repos: nil}
	results, err := s.Scan(context.Background(), "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty repos, got %v", results)
	}
}

func TestActionItemsEmpty(t *testing.T) {
	items := ActionItems(nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestActionItemsFromResults(t *testing.T) {
	results := []ScanResult{
		{
			Repo: "owner/repo1",
			Commits: []CommitSummary{
				{SHA: "abc1234", Message: "fix bug", Author: "dev"},
			},
			Issues: []IssueSummary{
				{Number: 42, Title: "Feature request", State: "open"},
				{Number: 43, Title: "Closed issue", State: "closed"},
			},
		},
		{
			Repo:    "owner/repo2",
			Commits: nil,
			Issues:  nil,
		},
	}

	items := ActionItems(results)
	if len(items) != 2 {
		t.Fatalf("expected 2 action items, got %d: %v", len(items), items)
	}
	// First: commit review
	if items[0] != "Review 1 new commits in owner/repo1" {
		t.Errorf("unexpected first item: %s", items[0])
	}
	// Second: open issue investigation
	if items[1] != "Investigate owner/repo1#42: Feature request" {
		t.Errorf("unexpected second item: %s", items[1])
	}
}

func TestTotalCommits(t *testing.T) {
	results := []ScanResult{
		{Commits: []CommitSummary{{}, {}}},
		{Commits: []CommitSummary{{}}},
	}
	if got := TotalCommits(results); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestTotalIssues(t *testing.T) {
	results := []ScanResult{
		{Issues: []IssueSummary{{}, {}}},
		{Issues: nil},
	}
	if got := TotalIssues(results); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}
