package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CommitSummary represents a commit from a GitHub repository scan.
type CommitSummary struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

// IssueSummary represents an issue from a GitHub repository scan.
type IssueSummary struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// ScanResult holds the scan data for a single repository.
type ScanResult struct {
	Repo    string          `json:"repo"`
	Commits []CommitSummary `json:"commits"`
	Issues  []IssueSummary  `json:"issues"`
}

// Scanner performs real GitHub API scans using the gh CLI.
type Scanner struct {
	Repos []string
}

// Scan queries GitHub for recent commits and issues across configured repos.
// Falls back gracefully with empty results if gh is not available.
func (s *Scanner) Scan(ctx context.Context, since string) ([]ScanResult, error) {
	if !ghAvailable(ctx) {
		return nil, nil
	}

	var results []ScanResult
	for _, repo := range s.Repos {
		result := ScanResult{Repo: repo}

		commits, err := fetchCommits(ctx, repo, since)
		if err == nil {
			result.Commits = commits
		}

		issues, err := fetchIssues(ctx, repo, since)
		if err == nil {
			result.Issues = issues
		}

		results = append(results, result)
	}
	return results, nil
}

// ActionItems derives actionable items from scan results.
func ActionItems(results []ScanResult) []string {
	var items []string
	for _, r := range results {
		if len(r.Commits) > 0 {
			items = append(items, fmt.Sprintf("Review %d new commits in %s", len(r.Commits), r.Repo))
		}
		for _, issue := range r.Issues {
			if issue.State == "open" {
				items = append(items, fmt.Sprintf("Investigate %s#%d: %s", r.Repo, issue.Number, issue.Title))
			}
		}
	}
	return items
}

// TotalCommits returns the total commit count across all results.
func TotalCommits(results []ScanResult) int {
	total := 0
	for _, r := range results {
		total += len(r.Commits)
	}
	return total
}

// TotalIssues returns the total issue count across all results.
func TotalIssues(results []ScanResult) int {
	total := 0
	for _, r := range results {
		total += len(r.Issues)
	}
	return total
}

// ghAvailable checks if the gh CLI is installed and accessible.
func ghAvailable(ctx context.Context) bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// ghCommitResult is used for JSON unmarshaling of gh api output.
type ghCommitResult struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
}

type ghIssueResult struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

func fetchCommits(ctx context.Context, repo, since string) ([]CommitSummary, error) {
	args := []string{"api", fmt.Sprintf("repos/%s/commits", repo), "--paginate", "-q", ".[].sha", "--jq", "."}
	if since != "" {
		args = []string{"api", fmt.Sprintf("repos/%s/commits?since=%s&per_page=30", repo, since), "--jq", "."}
	}

	out, err := exec.CommandContext(ctx, "gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetch commits for %s: %w", repo, err)
	}

	var raw []ghCommitResult
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse commits for %s: %w", repo, err)
	}

	var commits []CommitSummary
	for _, c := range raw {
		msg := c.Commit.Message
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		commits = append(commits, CommitSummary{
			SHA:     c.SHA[:min(7, len(c.SHA))],
			Message: msg,
			Author:  c.Commit.Author.Name,
		})
	}
	return commits, nil
}

func fetchIssues(ctx context.Context, repo, since string) ([]IssueSummary, error) {
	args := []string{"api", fmt.Sprintf("repos/%s/issues?state=open&since=%s&per_page=30", repo, since), "--jq", "."}

	out, err := exec.CommandContext(ctx, "gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetch issues for %s: %w", repo, err)
	}

	var raw []ghIssueResult
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse issues for %s: %w", repo, err)
	}

	var issues []IssueSummary
	for _, i := range raw {
		issues = append(issues, IssueSummary(i))
	}
	return issues, nil
}
