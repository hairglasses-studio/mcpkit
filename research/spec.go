package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// SpecInput is the input for the research_mcp_spec tool.
type SpecInput struct {
	FocusArea      string `json:"focus_area,omitempty" jsonschema:"description=Area to focus on (e.g. 'tools' or 'transport' or 'auth'). Leave empty to scan all areas."`
	IncludeRoadmap bool   `json:"include_roadmap,omitempty" jsonschema:"description=Whether to also fetch the MCP roadmap page (default false)"`
}

// SpecOutput is the output of the research_mcp_spec tool.
type SpecOutput struct {
	NewFeatures     []string        `json:"new_features"`
	ChangedFeatures []string        `json:"changed_features"`
	CoverageSummary CoverageSummary `json:"coverage_summary"`
	Confidence      ConfidenceLevel `json:"confidence"`
	Evidence        []string        `json:"evidence"`
}

// CoverageSummary summarizes mcpkit's spec coverage.
type CoverageSummary struct {
	Total       int    `json:"total"`
	Implemented int    `json:"implemented"`
	Partial     int    `json:"partial"`
	Missing     int    `json:"missing"`
	Percentage  string `json:"percentage"`
}

func (m *Module) specTool() registry.ToolDefinition {
	desc := "Check the MCP specification for changes against mcpkit's baseline feature matrix. " +
		"Fetches the spec page, keyword-matches against 14 tracked features, and reports new or changed features." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Scan all spec areas",
				Input:       map[string]any{},
				Output:      "Coverage summary with 10/14 features implemented",
			},
			{
				Description: "Focus on auth changes",
				Input:       map[string]any{"focus_area": "auth", "include_roadmap": true},
				Output:      "Auth-specific findings with roadmap context",
			},
		})

	return handler.TypedHandler[SpecInput, SpecOutput](
		"research_mcp_spec",
		desc,
		m.handleSpec,
	)
}

func (m *Module) handleSpec(ctx context.Context, input SpecInput) (SpecOutput, error) {
	out := SpecOutput{
		Confidence: ConfidenceMedium,
	}

	// Fetch spec page
	specURL := m.resolveURL("MCP Spec", defaultEcosystemSources[0].URL)
	specResult := m.fetchURL(ctx, specURL, 0)

	if specResult.Error != "" {
		out.Confidence = ConfidenceLow
		out.Evidence = append(out.Evidence, fmt.Sprintf("Spec fetch failed: %s", specResult.Error))
	} else {
		out.Evidence = append(out.Evidence, fmt.Sprintf("Fetched spec from %s (%d bytes)", specResult.URL, len(specResult.Body)))
		m.analyzeSpec(specResult.Body, input.FocusArea, &out)
	}

	// Optionally fetch roadmap
	if input.IncludeRoadmap {
		roadmapURL := m.resolveURL("MCP Roadmap", defaultEcosystemSources[1].URL)
		roadmapResult := m.fetchURL(ctx, roadmapURL, 0)
		if roadmapResult.Error != "" {
			out.Evidence = append(out.Evidence, fmt.Sprintf("Roadmap fetch failed: %s", roadmapResult.Error))
		} else {
			out.Evidence = append(out.Evidence, fmt.Sprintf("Fetched roadmap from %s (%d bytes)", roadmapResult.URL, len(roadmapResult.Body)))
			m.analyzeRoadmap(roadmapResult.Body, &out)
		}
	}

	// Compute coverage
	out.CoverageSummary = m.computeCoverage()

	return out, nil
}

func (m *Module) analyzeSpec(body, focusArea string, out *SpecOutput) {
	bodyLower := strings.ToLower(body)
	focusLower := strings.ToLower(focusArea)

	for _, feature := range m.baselineFeatures {
		nameLower := strings.ToLower(feature.Name)

		// If focus area is set, skip non-matching features
		if focusLower != "" && !strings.Contains(nameLower, focusLower) {
			continue
		}

		// Extract keywords from feature name
		keywords := extractKeywords(feature.Name)
		matches := 0
		for _, kw := range keywords {
			if strings.Contains(bodyLower, strings.ToLower(kw)) {
				matches++
			}
		}

		if matches == 0 && feature.Status == "Implemented" {
			out.ChangedFeatures = append(out.ChangedFeatures,
				fmt.Sprintf("%s: feature keywords not found in spec (may have been renamed or removed)", feature.Name))
		} else if matches > 0 && feature.Status == "Not implemented" {
			out.NewFeatures = append(out.NewFeatures,
				fmt.Sprintf("%s: found %d/%d keyword matches in spec", feature.Name, matches, len(keywords)))
		}
	}

	if len(out.NewFeatures) == 0 && len(out.ChangedFeatures) == 0 {
		out.Confidence = ConfidenceHigh
	}
}

func (m *Module) analyzeRoadmap(body string, out *SpecOutput) {
	bodyLower := strings.ToLower(body)

	roadmapKeywords := []string{"registry", "namespace", "agent-to-agent", "extensions", "gateway"}
	for _, kw := range roadmapKeywords {
		if strings.Contains(bodyLower, kw) {
			out.NewFeatures = append(out.NewFeatures,
				fmt.Sprintf("Roadmap mention: %q found in roadmap content", kw))
		}
	}
}

func (m *Module) computeCoverage() CoverageSummary {
	summary := CoverageSummary{Total: len(m.baselineFeatures)}

	for _, f := range m.baselineFeatures {
		switch f.Status {
		case "Implemented", "Delegated":
			summary.Implemented++
		case "Partial":
			summary.Partial++
		default:
			summary.Missing++
		}
	}

	pct := 0
	if summary.Total > 0 {
		pct = (summary.Implemented + summary.Partial) * 100 / summary.Total
	}
	summary.Percentage = fmt.Sprintf("%d%%", pct)

	return summary
}

func extractKeywords(name string) []string {
	// Extract meaningful words from feature name (skip common words)
	skip := map[string]bool{
		"and": true, "or": true, "the": true, "for": true, "a": true,
		"an": true, "in": true, "of": true, "to": true, "with": true,
	}

	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == ',' || r == '/'
	})

	var keywords []string
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if len(w) > 2 && !skip[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}
