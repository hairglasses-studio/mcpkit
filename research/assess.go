
package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// AssessInput is the input for the research_assess tool.
type AssessInput struct {
	Findings       []Finding      `json:"findings" jsonschema:"required,description=Findings to assess (from spec/sdk/ecosystem tools)"`
	ScoringWeights ScoringWeights `json:"scoring_weights,omitempty" jsonschema:"description=Custom weights for effort/impact/urgency scoring"`
}

// Finding is a single item to assess, typically produced by other research tools.
type Finding struct {
	Name        string `json:"name" jsonschema:"required,description=Name of the finding"`
	Category    string `json:"category,omitempty" jsonschema:"description=Category (spec_change/sdk_update/ecosystem/gap)"`
	Description string `json:"description,omitempty" jsonschema:"description=Details about the finding"`
	Severity    string `json:"severity,omitempty" jsonschema:"description=low/medium/high/critical"`
}

// ScoringWeights allows customizing the assessment heuristics.
type ScoringWeights struct {
	EffortWeight  float64 `json:"effort_weight,omitempty" jsonschema:"description=Weight for effort score (default 1.0)"`
	ImpactWeight  float64 `json:"impact_weight,omitempty" jsonschema:"description=Weight for impact score (default 1.5)"`
	UrgencyWeight float64 `json:"urgency_weight,omitempty" jsonschema:"description=Weight for urgency score (default 1.2)"`
}

// AssessOutput is the output of the research_assess tool.
type AssessOutput struct {
	Assessments     []Assessment `json:"assessments"`
	Recommendations []string     `json:"recommendations"`
	RiskFactors     []string     `json:"risk_factors"`
}

// Assessment is a scored evaluation of a single finding.
type Assessment struct {
	Name        string  `json:"name"`
	Effort      int     `json:"effort"`
	Impact      int     `json:"impact"`
	Urgency     int     `json:"urgency"`
	Priority    float64 `json:"priority"`
	Rationale   string  `json:"rationale"`
}

func (m *Module) assessTool() registry.ToolDefinition {
	desc := "Assess findings from research tools with effort/impact/urgency scoring. " +
		"Pure computation — no HTTP calls. Takes findings from spec, SDK, or ecosystem tools and produces " +
		"prioritized assessments with recommendations." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Assess a spec gap",
				Input: map[string]any{
					"findings": []any{
						map[string]any{"name": "Resources not implemented", "category": "gap", "severity": "high"},
					},
				},
				Output: "Prioritized assessment with effort=3, impact=5, urgency=4",
			},
		})

	return handler.TypedHandler[AssessInput, AssessOutput](
		"research_assess",
		desc,
		m.handleAssess,
	)
}

func (m *Module) handleAssess(_ context.Context, input AssessInput) (AssessOutput, error) {
	if len(input.Findings) == 0 {
		return AssessOutput{}, fmt.Errorf("at least one finding is required")
	}

	weights := normalizeWeights(input.ScoringWeights)
	out := AssessOutput{}

	for _, f := range input.Findings {
		a := scoreFinding(f, weights)
		out.Assessments = append(out.Assessments, a)
	}

	out.Recommendations = generateRecommendations(out.Assessments)
	out.RiskFactors = identifyRisks(input.Findings)

	return out, nil
}

func normalizeWeights(w ScoringWeights) ScoringWeights {
	if w.EffortWeight == 0 {
		w.EffortWeight = 1.0
	}
	if w.ImpactWeight == 0 {
		w.ImpactWeight = 1.5
	}
	if w.UrgencyWeight == 0 {
		w.UrgencyWeight = 1.2
	}
	return w
}

func scoreFinding(f Finding, w ScoringWeights) Assessment {
	a := Assessment{Name: f.Name}

	categoryLower := strings.ToLower(f.Category)
	severityLower := strings.ToLower(f.Severity)
	nameLower := strings.ToLower(f.Name)

	// Effort heuristic (1-5, higher = more effort)
	switch {
	case strings.Contains(nameLower, "implement") || strings.Contains(nameLower, "package"):
		a.Effort = 4
	case strings.Contains(nameLower, "upgrade") || strings.Contains(nameLower, "update"):
		a.Effort = 2
	case strings.Contains(nameLower, "fix") || strings.Contains(nameLower, "patch"):
		a.Effort = 1
	default:
		a.Effort = 3
	}

	// Impact heuristic (1-5, higher = more impact)
	switch severityLower {
	case "critical":
		a.Impact = 5
	case "high":
		a.Impact = 4
	case "medium":
		a.Impact = 3
	case "low":
		a.Impact = 2
	default:
		a.Impact = 3
	}

	// Urgency heuristic (1-5)
	switch categoryLower {
	case "spec_change":
		a.Urgency = 4
	case "sdk_update":
		a.Urgency = 3
	case "gap":
		a.Urgency = 4
	case "ecosystem":
		a.Urgency = 2
	default:
		a.Urgency = 3
	}

	// Boost for core MCP primitives
	if strings.Contains(nameLower, "resource") || strings.Contains(nameLower, "prompt") {
		a.Impact++
		if a.Impact > 5 {
			a.Impact = 5
		}
	}

	// Composite priority: (impact * weight + urgency * weight) / (effort * weight)
	a.Priority = (float64(a.Impact)*w.ImpactWeight + float64(a.Urgency)*w.UrgencyWeight) /
		(float64(a.Effort) * w.EffortWeight)

	a.Rationale = fmt.Sprintf("effort=%d impact=%d urgency=%d (category=%s, severity=%s)",
		a.Effort, a.Impact, a.Urgency, f.Category, f.Severity)

	return a
}

func generateRecommendations(assessments []Assessment) []string {
	var recs []string

	// Find highest priority
	var maxPriority float64
	var topName string
	for _, a := range assessments {
		if a.Priority > maxPriority {
			maxPriority = a.Priority
			topName = a.Name
		}
	}

	if topName != "" {
		recs = append(recs, fmt.Sprintf("Highest priority: %s (score %.2f)", topName, maxPriority))
	}

	// Count high-effort items
	highEffort := 0
	for _, a := range assessments {
		if a.Effort >= 4 {
			highEffort++
		}
	}
	if highEffort > 0 {
		recs = append(recs, fmt.Sprintf("%d item(s) require significant effort (4+); consider phased implementation", highEffort))
	}

	return recs
}

func identifyRisks(findings []Finding) []string {
	var risks []string

	criticalCount := 0
	for _, f := range findings {
		if strings.EqualFold(f.Severity, "critical") {
			criticalCount++
		}
	}
	if criticalCount > 0 {
		risks = append(risks, fmt.Sprintf("%d critical finding(s) require immediate attention", criticalCount))
	}

	specChanges := 0
	for _, f := range findings {
		if strings.EqualFold(f.Category, "spec_change") {
			specChanges++
		}
	}
	if specChanges > 0 {
		risks = append(risks, fmt.Sprintf("%d spec change(s) detected — may affect existing implementations", specChanges))
	}

	if len(risks) == 0 {
		risks = append(risks, "No critical risks identified")
	}

	return risks
}
