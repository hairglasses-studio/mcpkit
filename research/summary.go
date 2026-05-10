package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// SummaryInput accepts outputs from all other research tools.
type SummaryInput struct {
	SpecFindings        *SpecOutput                 `json:"spec_findings,omitempty" jsonschema:"description=Output from research_mcp_spec tool"`
	SDKFindings         *SDKOutput                  `json:"sdk_findings,omitempty" jsonschema:"description=Output from research_sdk_releases tool"`
	EcosystemFindings   *EcosystemOutput            `json:"ecosystem_findings,omitempty" jsonschema:"description=Output from research_ecosystem tool"`
	PlatformFindings    *PlatformOutput             `json:"platform_findings,omitempty" jsonschema:"description=Output from research_platform_activity tool"`
	A2AFindings         *A2ATrackingOutput          `json:"a2a_findings,omitempty" jsonschema:"description=Output from research_a2a_tracking tool"`
	SDKCompareFindings  *SDKCompareOutput           `json:"sdk_compare_findings,omitempty" jsonschema:"description=Output from research_sdk_compare tool"`
	CompetitiveFindings *CompetitiveDashboardOutput `json:"competitive_findings,omitempty" jsonschema:"description=Output from research_competitive_dashboard tool"`
	AssessFindings      *AssessOutput               `json:"assess_findings,omitempty" jsonschema:"description=Output from research_assess tool"`
	AdditionalNotes     string                      `json:"additional_notes,omitempty" jsonschema:"description=Free-form notes to include in the summary"`
	OutputFormat        string                      `json:"output_format,omitempty" jsonschema:"description=Output format: 'markdown' (default) or 'json',enum=markdown,enum=json"`
}

// SummaryOutput is the combined research summary.
type SummaryOutput struct {
	Report               string         `json:"report"`
	Sections             []Section      `json:"sections"`
	ActionItems          []string       `json:"action_items"`
	UpdatedFeatureMatrix []FeatureEntry `json:"updated_feature_matrix"`
}

// Section is a named section of the summary report.
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (m *Module) summaryTool() registry.ToolDefinition {
	desc := "Combine outputs from all research tools into a unified summary report. " +
		"Accepts spec, SDK, ecosystem, platform, A2A, competitive, and assessment findings and produces a markdown report " +
		"with action items and an updated feature matrix." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Generate summary from spec and SDK findings",
				Input: map[string]any{
					"spec_findings": map[string]any{"coverage_summary": map[string]any{"percentage": "72%"}},
					"sdk_findings":  map[string]any{"go_mod_version": "v0.45.0"},
				},
				Output: "Markdown report with sections, action items, and updated feature matrix",
			},
		})

	return handler.TypedHandler[SummaryInput, SummaryOutput](
		"research_summary",
		desc,
		m.handleSummary,
	)
}

func (m *Module) handleSummary(_ context.Context, input SummaryInput) (SummaryOutput, error) {
	out := SummaryOutput{
		UpdatedFeatureMatrix: make([]FeatureEntry, len(m.baselineFeatures)),
	}
	copy(out.UpdatedFeatureMatrix, m.baselineFeatures)

	var sections []Section
	var actions []string

	// Spec section
	if input.SpecFindings != nil {
		sf := input.SpecFindings
		var content strings.Builder
		fmt.Fprintf(&content, "Coverage: %s (%d/%d implemented, %d partial, %d missing)\n",
			sf.CoverageSummary.Percentage,
			sf.CoverageSummary.Implemented,
			sf.CoverageSummary.Total,
			sf.CoverageSummary.Partial,
			sf.CoverageSummary.Missing)

		if len(sf.NewFeatures) > 0 {
			content.WriteString("\nNew features detected:\n")
			for _, f := range sf.NewFeatures {
				fmt.Fprintf(&content, "- %s\n", f)
			}
		}
		if len(sf.ChangedFeatures) > 0 {
			content.WriteString("\nChanged features:\n")
			for _, f := range sf.ChangedFeatures {
				fmt.Fprintf(&content, "- %s\n", f)
				actions = append(actions, fmt.Sprintf("Investigate spec change: %s", f))
			}
		}

		sections = append(sections, Section{Title: "MCP Specification", Content: content.String()})
	}

	// SDK section
	if input.SDKFindings != nil {
		sf := input.SDKFindings
		var content strings.Builder
		fmt.Fprintf(&content, "Current mcp-go: %s\n", sf.GoModVersion)
		if sf.LatestUpstream != "" {
			fmt.Fprintf(&content, "Latest upstream: %s\n", sf.LatestUpstream)
		}

		for _, repo := range sf.Repos {
			if repo.Error != "" {
				fmt.Fprintf(&content, "\n%s/%s: ERROR - %s\n", repo.Owner, repo.Repo, repo.Error)
			} else {
				fmt.Fprintf(&content, "\n%s/%s: latest=%s\n", repo.Owner, repo.Repo, repo.LatestTag)
			}
		}

		actions = append(actions, sf.UpgradeAdvice...)

		sections = append(sections, Section{Title: "SDK Releases", Content: content.String()})
	}

	// Ecosystem section
	if input.EcosystemFindings != nil {
		ef := input.EcosystemFindings
		var content strings.Builder

		for _, src := range ef.Sources {
			if src.Error != "" {
				fmt.Fprintf(&content, "- %s: ERROR - %s\n", src.Name, src.Error)
			} else {
				fmt.Fprintf(&content, "- %s: relevance=%.0f%% (%d keyword hits)\n",
					src.Name, src.Relevance*100, len(src.KeywordHits))
			}
		}

		if len(ef.Highlights) > 0 {
			content.WriteString("\nHighlights:\n")
			for _, h := range ef.Highlights {
				fmt.Fprintf(&content, "- [%s] %s\n", h.Source, h.Text)
			}
		}

		sections = append(sections, Section{Title: "Ecosystem", Content: content.String()})
	}

	// Platform section
	if input.PlatformFindings != nil {
		pf := input.PlatformFindings
		var content strings.Builder
		if pf.Summary != "" {
			fmt.Fprintf(&content, "%s\n", pf.Summary)
		}
		for _, result := range pf.Results {
			if result.Error != "" {
				fmt.Fprintf(&content, "- %s: ERROR - %s\n", result.Platform, result.Error)
				continue
			}
			fmt.Fprintf(&content, "- %s: relevance=%.0f%% (%d sources, %d signals)\n",
				result.Platform, result.Relevance*100, len(result.Sources), len(result.Signals))
		}
		if len(pf.Signals) > 0 {
			content.WriteString("\nSignals:\n")
			for _, signal := range pf.Signals {
				fmt.Fprintf(&content, "- [%s/%s] %s - %s\n", signal.Platform, signal.Severity, signal.Keyword, signal.Snippet)
			}
		}
		actions = append(actions, pf.ActionItems...)
		sections = append(sections, Section{Title: "Cloud Platforms", Content: content.String()})
	}

	// A2A section
	if input.A2AFindings != nil {
		af := input.A2AFindings
		var content strings.Builder
		if af.Summary != "" {
			fmt.Fprintf(&content, "%s\n", af.Summary)
		}
		if len(af.VersionCandidates) > 0 {
			fmt.Fprintf(&content, "Detected versions: %s\n", strings.Join(af.VersionCandidates, ", "))
		}
		for _, signal := range af.Signals {
			fmt.Fprintf(&content, "- [%s] %s from %s - %s\n", signal.Severity, signal.Keyword, signal.Source, signal.Snippet)
		}
		actions = append(actions, af.ActionItems...)
		sections = append(sections, Section{Title: "A2A Tracking", Content: content.String()})
	}

	// SDK comparison section
	if input.SDKCompareFindings != nil {
		sf := input.SDKCompareFindings
		var content strings.Builder
		if sf.Summary != "" {
			fmt.Fprintf(&content, "%s\n", sf.Summary)
		}
		for _, row := range sf.Matrix {
			var parts []string
			for _, support := range row.Supports {
				parts = append(parts, fmt.Sprintf("%s=%s", support.SDK, support.Status))
			}
			fmt.Fprintf(&content, "- %s: %s\n", row.Feature, strings.Join(parts, ", "))
		}
		actions = append(actions, sf.ActionItems...)
		sections = append(sections, Section{Title: "SDK Comparison", Content: content.String()})
	}

	// Competitive dashboard section
	if input.CompetitiveFindings != nil {
		cf := input.CompetitiveFindings
		var content strings.Builder
		if cf.Summary != "" {
			fmt.Fprintf(&content, "%s\n", cf.Summary)
		}
		for _, row := range cf.Rows {
			fmt.Fprintf(&content, "- [%s/%s] %s: %s\n", row.Area, row.Severity, row.Subject, row.Status)
		}
		actions = append(actions, cf.ActionItems...)
		sections = append(sections, Section{Title: "Competitive Dashboard", Content: content.String()})
	}

	// Assessment section
	if input.AssessFindings != nil {
		af := input.AssessFindings
		var content strings.Builder

		for _, a := range af.Assessments {
			fmt.Fprintf(&content, "- %s: priority=%.2f (%s)\n", a.Name, a.Priority, a.Rationale)
		}

		actions = append(actions, af.Recommendations...)
		for _, risk := range af.RiskFactors {
			fmt.Fprintf(&content, "\nRisk: %s\n", risk)
		}

		sections = append(sections, Section{Title: "Assessment", Content: content.String()})
	}

	// Additional notes
	if input.AdditionalNotes != "" {
		sections = append(sections, Section{Title: "Notes", Content: input.AdditionalNotes})
	}

	out.Sections = sections
	out.ActionItems = actions

	// Build markdown report
	out.Report = buildMarkdownReport(sections, actions)

	return out, nil
}

func buildMarkdownReport(sections []Section, actions []string) string {
	var b strings.Builder

	b.WriteString("# MCP Ecosystem Research Summary\n\n")

	for _, s := range sections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Title, s.Content)
	}

	if len(actions) > 0 {
		b.WriteString("## Action Items\n\n")
		for i, a := range actions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, a)
		}
		b.WriteString("\n")
	}

	return b.String()
}
