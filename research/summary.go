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
	SpecFindings      *SpecOutput      `json:"spec_findings,omitempty" jsonschema:"description=Output from research_mcp_spec tool"`
	SDKFindings       *SDKOutput       `json:"sdk_findings,omitempty" jsonschema:"description=Output from research_sdk_releases tool"`
	EcosystemFindings *EcosystemOutput `json:"ecosystem_findings,omitempty" jsonschema:"description=Output from research_ecosystem tool"`
	AssessFindings    *AssessOutput    `json:"assess_findings,omitempty" jsonschema:"description=Output from research_assess tool"`
	AdditionalNotes   string           `json:"additional_notes,omitempty" jsonschema:"description=Free-form notes to include in the summary"`
	OutputFormat      string           `json:"output_format,omitempty" jsonschema:"description=Output format: 'markdown' (default) or 'json',enum=markdown,enum=json"`
}

// SummaryOutput is the combined research summary.
type SummaryOutput struct {
	Report              string         `json:"report"`
	Sections            []Section      `json:"sections"`
	ActionItems         []string       `json:"action_items"`
	UpdatedFeatureMatrix []FeatureEntry `json:"updated_feature_matrix"`
}

// Section is a named section of the summary report.
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (m *Module) summaryTool() registry.ToolDefinition {
	desc := "Combine outputs from all research tools into a unified summary report. " +
		"Accepts spec, SDK, ecosystem, and assessment findings and produces a markdown report " +
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
		content.WriteString(fmt.Sprintf("Coverage: %s (%d/%d implemented, %d partial, %d missing)\n",
			sf.CoverageSummary.Percentage,
			sf.CoverageSummary.Implemented,
			sf.CoverageSummary.Total,
			sf.CoverageSummary.Partial,
			sf.CoverageSummary.Missing))

		if len(sf.NewFeatures) > 0 {
			content.WriteString("\nNew features detected:\n")
			for _, f := range sf.NewFeatures {
				content.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
		if len(sf.ChangedFeatures) > 0 {
			content.WriteString("\nChanged features:\n")
			for _, f := range sf.ChangedFeatures {
				content.WriteString(fmt.Sprintf("- %s\n", f))
				actions = append(actions, fmt.Sprintf("Investigate spec change: %s", f))
			}
		}

		sections = append(sections, Section{Title: "MCP Specification", Content: content.String()})
	}

	// SDK section
	if input.SDKFindings != nil {
		sf := input.SDKFindings
		var content strings.Builder
		content.WriteString(fmt.Sprintf("Current mcp-go: %s\n", sf.GoModVersion))
		if sf.LatestUpstream != "" {
			content.WriteString(fmt.Sprintf("Latest upstream: %s\n", sf.LatestUpstream))
		}

		for _, repo := range sf.Repos {
			if repo.Error != "" {
				content.WriteString(fmt.Sprintf("\n%s/%s: ERROR - %s\n", repo.Owner, repo.Repo, repo.Error))
			} else {
				content.WriteString(fmt.Sprintf("\n%s/%s: latest=%s\n", repo.Owner, repo.Repo, repo.LatestTag))
			}
		}

		for _, advice := range sf.UpgradeAdvice {
			actions = append(actions, advice)
		}

		sections = append(sections, Section{Title: "SDK Releases", Content: content.String()})
	}

	// Ecosystem section
	if input.EcosystemFindings != nil {
		ef := input.EcosystemFindings
		var content strings.Builder

		for _, src := range ef.Sources {
			if src.Error != "" {
				content.WriteString(fmt.Sprintf("- %s: ERROR - %s\n", src.Name, src.Error))
			} else {
				content.WriteString(fmt.Sprintf("- %s: relevance=%.0f%% (%d keyword hits)\n",
					src.Name, src.Relevance*100, len(src.KeywordHits)))
			}
		}

		if len(ef.Highlights) > 0 {
			content.WriteString("\nHighlights:\n")
			for _, h := range ef.Highlights {
				content.WriteString(fmt.Sprintf("- [%s] %s\n", h.Source, h.Text))
			}
		}

		sections = append(sections, Section{Title: "Ecosystem", Content: content.String()})
	}

	// Assessment section
	if input.AssessFindings != nil {
		af := input.AssessFindings
		var content strings.Builder

		for _, a := range af.Assessments {
			content.WriteString(fmt.Sprintf("- %s: priority=%.2f (%s)\n", a.Name, a.Priority, a.Rationale))
		}

		for _, rec := range af.Recommendations {
			actions = append(actions, rec)
		}
		for _, risk := range af.RiskFactors {
			content.WriteString(fmt.Sprintf("\nRisk: %s\n", risk))
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
		b.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", s.Title, s.Content))
	}

	if len(actions) > 0 {
		b.WriteString("## Action Items\n\n")
		for i, a := range actions {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, a))
		}
		b.WriteString("\n")
	}

	return b.String()
}
