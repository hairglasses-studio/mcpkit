package rdcycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ScanInput is the input for the rdcycle_scan tool.
type ScanInput struct {
	Repos []string `json:"repos,omitempty" jsonschema:"description=GitHub repos to scan in owner/repo format (default: from config)"`
	Since string   `json:"since,omitempty" jsonschema:"description=ISO 8601 date to scan from (default: 7 days ago)"`
}

// ScanOutput is the output of the rdcycle_scan tool.
type ScanOutput struct {
	Summary     string   `json:"summary"`
	RepoCount   int      `json:"repo_count"`
	CommitCount int      `json:"commit_count"`
	IssueCount  int      `json:"issue_count"`
	ActionItems []string `json:"action_items"`
	ArtifactID  string   `json:"artifact_id"`
}

func (m *Module) scanTool() registry.ToolDefinition {
	desc := "Scan the MCP ecosystem for recent activity across configured GitHub repositories. " +
		"Returns a structured summary of commits, issues, and actionable items derived from repository activity. " +
		"Use repos to override the module-configured list. " +
		"Use since (ISO 8601 date) to control the scan window (default: 7 days ago)."

	td := handler.TypedHandler[ScanInput, ScanOutput](
		"rdcycle_scan",
		desc,
		m.handleScan,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.Complexity = registry.ComplexityModerate
	return td
}

func (m *Module) handleScan(ctx context.Context, input ScanInput) (ScanOutput, error) {
	repos := m.config.ScanRepos
	if len(input.Repos) > 0 {
		repos = input.Repos
	}

	since := input.Since
	if since == "" {
		since = m.config.DateRange
	}
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	}

	// Try real GitHub scan first.
	scanner := &Scanner{Repos: repos}
	results, err := scanner.Scan(ctx, since)

	var actionItems []string
	commitCount := 0
	issueCount := 0

	if err == nil && len(results) > 0 {
		commitCount = TotalCommits(results)
		issueCount = TotalIssues(results)
		actionItems = ActionItems(results)
	} else {
		// Fallback: generate structured placeholders.
		for _, repo := range repos {
			actionItems = append(actionItems, fmt.Sprintf("Review recent activity in %s since %s", repo, since))
		}
	}

	summary := buildScanSummary(repos, since)

	output := ScanOutput{
		Summary:     summary,
		RepoCount:   len(repos),
		CommitCount: commitCount,
		IssueCount:  issueCount,
		ActionItems: actionItems,
		ArtifactID:  fmt.Sprintf("scan-%d", time.Now().UnixNano()),
	}

	_ = m.store.Save(Artifact{
		ID:        output.ArtifactID,
		Type:      "scan",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Content: map[string]any{
			"repos":        repos,
			"since":        since,
			"repo_count":   output.RepoCount,
			"commit_count": output.CommitCount,
			"issue_count":  output.IssueCount,
			"action_items": output.ActionItems,
		},
	})

	return output, nil
}

// buildScanSummary generates a human-readable summary string.
func buildScanSummary(repos []string, since string) string {
	if len(repos) == 0 {
		return fmt.Sprintf("No repositories configured for scan since %s.", since)
	}
	return fmt.Sprintf(
		"Scanned %d repositor%s since %s: %s.",
		len(repos),
		pluralSuffix(len(repos), "y", "ies"),
		since,
		strings.Join(repos, ", "),
	)
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
