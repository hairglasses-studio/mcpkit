package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	Summary         string                 `json:"summary"`
	RepoCount       int                    `json:"repo_count"`
	CommitCount     int                    `json:"commit_count"`
	IssueCount      int                    `json:"issue_count"`
	ActionItems     []string               `json:"action_items"`
	FeedbackSignals *FeedbackSignalSummary `json:"feedback_signals,omitempty"`
	ResearchSignals *ResearchSignalSummary `json:"research_signals,omitempty"`
	ArtifactID      string                 `json:"artifact_id"`
}

// FeedbackSignalSummary summarizes opt-in feedback telemetry for rdcycle scans.
type FeedbackSignalSummary struct {
	Enabled     bool                   `json:"enabled"`
	Source      string                 `json:"source,omitempty"`
	TotalCalls  int64                  `json:"total_calls"`
	TotalErrors int64                  `json:"total_errors"`
	ErrorRate   float64                `json:"error_rate"`
	TopTargets  []FeedbackSignalTarget `json:"top_targets,omitempty"`
}

// FeedbackSignalTarget is one high-signal feedback telemetry aggregate.
type FeedbackSignalTarget struct {
	Target    string  `json:"target"`
	Package   string  `json:"package"`
	Tool      string  `json:"tool"`
	Calls     int64   `json:"calls"`
	Errors    int64   `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
}

// ResearchSignalSummary summarizes competitive research output for rdcycle scans.
type ResearchSignalSummary struct {
	Enabled            bool                `json:"enabled"`
	Summary            string              `json:"summary,omitempty"`
	A2ASignals         int                 `json:"a2a_signals"`
	A2ABreakingSignals int                 `json:"a2a_breaking_signals"`
	SDKFrameworks      int                 `json:"sdk_frameworks"`
	SDKFeatures        int                 `json:"sdk_features"`
	ActionItems        []string            `json:"action_items,omitempty"`
	TopRows            []ResearchSignalRow `json:"top_rows,omitempty"`
	Error              string              `json:"error,omitempty"`
}

// ResearchSignalRow is one normalized competitive dashboard row.
type ResearchSignalRow struct {
	Area     string  `json:"area"`
	Subject  string  `json:"subject"`
	Status   string  `json:"status"`
	Severity string  `json:"severity"`
	Details  string  `json:"details"`
	Score    float64 `json:"score"`
}

type feedbackDashboardExport struct {
	Source string                 `json:"source,omitempty"`
	Rows   []feedbackDashboardRow `json:"rows"`
}

type feedbackDashboardRow struct {
	Target    string  `json:"target"`
	Package   string  `json:"package"`
	Tool      string  `json:"tool"`
	Calls     int64   `json:"calls"`
	Errors    int64   `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
}

type researchDashboardExport struct {
	Summary       string                       `json:"summary"`
	A2ASignals    int                          `json:"a2a_signals"`
	SDKFrameworks int                          `json:"sdk_frameworks"`
	SDKFeatures   int                          `json:"sdk_features"`
	Rows          []researchDashboardExportRow `json:"rows"`
	ActionItems   []string                     `json:"action_items"`
}

type researchDashboardExportRow struct {
	Area     string  `json:"area"`
	Subject  string  `json:"subject"`
	Status   string  `json:"status"`
	Severity string  `json:"severity"`
	Details  string  `json:"details"`
	Score    float64 `json:"score"`
}

func (m *Module) scanTool() registry.ToolDefinition {
	desc := "Scan the MCP ecosystem for recent activity across configured GitHub repositories. " +
		"Returns a structured summary of commits, issues, and actionable items derived from repository activity. " +
		"Includes optional feedback and competitive research signals when providers are configured. " +
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

	feedbackSignals := m.feedbackSignalSummary()
	actionItems = append(actionItems, feedbackActionItems(feedbackSignals)...)
	researchSignals := m.researchSignalSummary(ctx)
	actionItems = append(actionItems, researchActionItems(researchSignals)...)

	summary := buildScanSummary(repos, since)

	output := ScanOutput{
		Summary:         summary,
		RepoCount:       len(repos),
		CommitCount:     commitCount,
		IssueCount:      issueCount,
		ActionItems:     actionItems,
		FeedbackSignals: feedbackSignals,
		ResearchSignals: researchSignals,
		ArtifactID:      fmt.Sprintf("scan-%d", time.Now().UnixNano()),
	}

	content := map[string]any{
		"repos":        repos,
		"since":        since,
		"repo_count":   output.RepoCount,
		"commit_count": output.CommitCount,
		"issue_count":  output.IssueCount,
		"action_items": output.ActionItems,
	}
	if output.FeedbackSignals != nil {
		content["feedback_signals"] = output.FeedbackSignals
	}
	if output.ResearchSignals != nil {
		content["research_signals"] = output.ResearchSignals
	}
	_ = m.store.Save(Artifact{
		ID:        output.ArtifactID,
		Type:      "scan",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Content:   content,
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

func (m *Module) feedbackSignalSummary() *FeedbackSignalSummary {
	if m.feedback == nil {
		return nil
	}
	data, err := m.feedback.DashboardJSON()
	if err != nil {
		return &FeedbackSignalSummary{Enabled: true}
	}
	var export feedbackDashboardExport
	if err := json.Unmarshal(data, &export); err != nil {
		return &FeedbackSignalSummary{Enabled: true}
	}
	return feedbackSignalSummary(export)
}

func feedbackSignalSummary(export feedbackDashboardExport) *FeedbackSignalSummary {
	summary := &FeedbackSignalSummary{
		Enabled: true,
		Source:  export.Source,
	}
	for _, row := range export.Rows {
		summary.TotalCalls += row.Calls
		summary.TotalErrors += row.Errors
		summary.TopTargets = append(summary.TopTargets, FeedbackSignalTarget{
			Target:    row.Target,
			Package:   row.Package,
			Tool:      row.Tool,
			Calls:     row.Calls,
			Errors:    row.Errors,
			ErrorRate: row.ErrorRate,
		})
	}
	if summary.TotalCalls > 0 {
		summary.ErrorRate = float64(summary.TotalErrors) / float64(summary.TotalCalls)
	}
	sort.Slice(summary.TopTargets, func(i, j int) bool {
		a, b := summary.TopTargets[i], summary.TopTargets[j]
		if a.Errors != b.Errors {
			return a.Errors > b.Errors
		}
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Target < b.Target
	})
	if len(summary.TopTargets) > 5 {
		summary.TopTargets = summary.TopTargets[:5]
	}
	return summary
}

func feedbackActionItems(summary *FeedbackSignalSummary) []string {
	if summary == nil || summary.TotalErrors == 0 {
		return nil
	}
	items := []string{
		fmt.Sprintf("Review feedback telemetry: %d errors across %d calls (%.1f%% error rate).",
			summary.TotalErrors, summary.TotalCalls, summary.ErrorRate*100),
	}
	if len(summary.TopTargets) > 0 && summary.TopTargets[0].Errors > 0 {
		top := summary.TopTargets[0]
		items = append(items, fmt.Sprintf("Prioritize feedback target %s: %d errors across %d calls.",
			top.Target, top.Errors, top.Calls))
	}
	return items
}

func (m *Module) researchSignalSummary(ctx context.Context) *ResearchSignalSummary {
	if m.research == nil {
		return nil
	}
	data, err := m.research.DashboardJSON(ctx)
	if err != nil {
		return &ResearchSignalSummary{Enabled: true, Error: err.Error()}
	}
	var export researchDashboardExport
	if err := json.Unmarshal(data, &export); err != nil {
		return &ResearchSignalSummary{Enabled: true, Error: err.Error()}
	}
	return researchSignalSummary(export)
}

func researchSignalSummary(export researchDashboardExport) *ResearchSignalSummary {
	summary := &ResearchSignalSummary{
		Enabled:       true,
		Summary:       export.Summary,
		A2ASignals:    export.A2ASignals,
		SDKFrameworks: export.SDKFrameworks,
		SDKFeatures:   export.SDKFeatures,
		ActionItems:   dedupeScanStrings(export.ActionItems),
	}
	for _, row := range export.Rows {
		if row.Area == "a2a" && row.Severity == "breaking" {
			summary.A2ABreakingSignals++
		}
		summary.TopRows = append(summary.TopRows, ResearchSignalRow{
			Area:     row.Area,
			Subject:  row.Subject,
			Status:   row.Status,
			Severity: row.Severity,
			Details:  row.Details,
			Score:    row.Score,
		})
	}
	sort.Slice(summary.TopRows, func(i, j int) bool {
		a, b := summary.TopRows[i], summary.TopRows[j]
		if a.Severity != b.Severity {
			return researchSeverityRank(a.Severity) > researchSeverityRank(b.Severity)
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Subject < b.Subject
	})
	if len(summary.TopRows) > 8 {
		summary.TopRows = summary.TopRows[:8]
	}
	return summary
}

func researchActionItems(summary *ResearchSignalSummary) []string {
	if summary == nil {
		return nil
	}
	if summary.Error != "" {
		return []string{fmt.Sprintf("Restore competitive research signal provider: %s", summary.Error)}
	}
	items := append([]string(nil), summary.ActionItems...)
	if summary.A2ABreakingSignals > 0 {
		items = append(items, fmt.Sprintf("Prioritize A2A compatibility review: %d breaking signal%s detected.",
			summary.A2ABreakingSignals, pluralSuffix(summary.A2ABreakingSignals, "", "s")))
	}
	return dedupeScanStrings(items)
}

func researchSeverityRank(severity string) int {
	switch severity {
	case "breaking":
		return 4
	case "high":
		return 3
	case "agent-card", "version", "medium":
		return 2
	default:
		return 1
	}
}

func dedupeScanStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
