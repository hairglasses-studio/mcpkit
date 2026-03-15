
package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// DiffInput is the input for the research_diff_analysis tool.
type DiffInput struct {
	Before SummaryOutput `json:"before" jsonschema:"required,description=Previous summary snapshot"`
	After  SummaryOutput `json:"after" jsonschema:"required,description=Current summary snapshot"`
}

// DiffReport is the output of the research_diff_analysis tool.
type DiffReport struct {
	NewActionItems  []string        `json:"new_action_items"`
	ResolvedItems   []string        `json:"resolved_items"`
	ChangedSections []SectionDiff   `json:"changed_sections"`
	FeatureChanges  []FeatureChange `json:"feature_changes"`
	Summary         string          `json:"summary"`
}

// SectionDiff describes a change to a named section.
type SectionDiff struct {
	Title  string `json:"title"`
	Change string `json:"change"` // added, removed, modified
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// FeatureChange describes a status transition for a feature matrix entry.
type FeatureChange struct {
	Name      string `json:"name"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
}

func (m *Module) diffAnalysisTool() registry.ToolDefinition {
	desc := "Compare two SummaryOutput snapshots and produce a diff report. " +
		"Pure computation — no HTTP calls. Identifies new/resolved action items, " +
		"added/removed/modified sections, and feature matrix status transitions." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Compare two weekly research summaries",
				Input: map[string]any{
					"before": map[string]any{
						"action_items": []any{"Upgrade mcp-go to v0.47.0"},
						"sections":     []any{},
					},
					"after": map[string]any{
						"action_items": []any{"Investigate spec change: elicitation renamed"},
						"sections":     []any{},
					},
				},
				Output: "DiffReport with 1 new item, 1 resolved item",
			},
		})

	return handler.TypedHandler[DiffInput, DiffReport](
		"research_diff_analysis",
		desc,
		m.handleDiffAnalysis,
	)
}

func (m *Module) handleDiffAnalysis(_ context.Context, input DiffInput) (DiffReport, error) {
	report := DiffReport{}

	// Diff action items
	report.NewActionItems, report.ResolvedItems = diffStringSlices(input.Before.ActionItems, input.After.ActionItems)

	// Diff sections by title
	report.ChangedSections = diffSections(input.Before.Sections, input.After.Sections)

	// Diff feature matrix by name
	report.FeatureChanges = diffFeatureMatrix(input.Before.UpdatedFeatureMatrix, input.After.UpdatedFeatureMatrix)

	// Build summary
	report.Summary = buildDiffSummary(report)

	return report, nil
}

// diffStringSlices returns items in after but not before (newItems) and items in
// before but not after (resolvedItems).
func diffStringSlices(before, after []string) (newItems, resolvedItems []string) {
	beforeSet := make(map[string]struct{}, len(before))
	for _, s := range before {
		beforeSet[s] = struct{}{}
	}

	afterSet := make(map[string]struct{}, len(after))
	for _, s := range after {
		afterSet[s] = struct{}{}
	}

	for _, s := range after {
		if _, ok := beforeSet[s]; !ok {
			newItems = append(newItems, s)
		}
	}

	for _, s := range before {
		if _, ok := afterSet[s]; !ok {
			resolvedItems = append(resolvedItems, s)
		}
	}

	return newItems, resolvedItems
}

// diffSections compares section slices by title, returning SectionDiff entries
// for added, removed, and modified sections.
func diffSections(before, after []Section) []SectionDiff {
	beforeMap := make(map[string]string, len(before))
	for _, s := range before {
		beforeMap[s.Title] = s.Content
	}

	afterMap := make(map[string]string, len(after))
	for _, s := range after {
		afterMap[s.Title] = s.Content
	}

	var diffs []SectionDiff

	// Added or modified
	for _, s := range after {
		beforeContent, existed := beforeMap[s.Title]
		if !existed {
			diffs = append(diffs, SectionDiff{
				Title:  s.Title,
				Change: "added",
				After:  s.Content,
			})
		} else if beforeContent != s.Content {
			diffs = append(diffs, SectionDiff{
				Title:  s.Title,
				Change: "modified",
				Before: beforeContent,
				After:  s.Content,
			})
		}
	}

	// Removed
	for _, s := range before {
		if _, ok := afterMap[s.Title]; !ok {
			diffs = append(diffs, SectionDiff{
				Title:  s.Title,
				Change: "removed",
				Before: s.Content,
			})
		}
	}

	return diffs
}

// diffFeatureMatrix compares feature matrix entries by name, returning FeatureChange
// entries where the status changed.
func diffFeatureMatrix(before, after []FeatureEntry) []FeatureChange {
	beforeMap := make(map[string]string, len(before))
	for _, f := range before {
		beforeMap[f.Name] = f.Status
	}

	var changes []FeatureChange
	for _, f := range after {
		oldStatus, existed := beforeMap[f.Name]
		if existed && oldStatus != f.Status {
			changes = append(changes, FeatureChange{
				Name:      f.Name,
				OldStatus: oldStatus,
				NewStatus: f.Status,
			})
		}
	}
	return changes
}

func buildDiffSummary(r DiffReport) string {
	parts := []string{}

	if len(r.NewActionItems) > 0 {
		parts = append(parts, fmt.Sprintf("%d new action item(s)", len(r.NewActionItems)))
	}
	if len(r.ResolvedItems) > 0 {
		parts = append(parts, fmt.Sprintf("%d resolved item(s)", len(r.ResolvedItems)))
	}
	if len(r.ChangedSections) > 0 {
		parts = append(parts, fmt.Sprintf("%d section change(s)", len(r.ChangedSections)))
	}
	if len(r.FeatureChanges) > 0 {
		parts = append(parts, fmt.Sprintf("%d feature status change(s)", len(r.FeatureChanges)))
	}

	if len(parts) == 0 {
		return "No changes detected between snapshots"
	}

	return strings.Join(parts, ", ")
}
