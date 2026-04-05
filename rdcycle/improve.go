package rdcycle

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ImproveInput is the input for the rdcycle_improve tool.
type ImproveInput struct {
	NotesPath string `json:"notes_path,omitempty" jsonschema:"description=Path to improvement log (default: rdcycle/notes/improvement_log.json)"`
}

// ImproveOutput is the output of the rdcycle_improve tool.
type ImproveOutput struct {
	CyclesAnalyzed   int            `json:"cycles_analyzed"`
	CommonFailures   []string       `json:"common_failures"`
	AvgWastedIters   float64        `json:"avg_wasted_iterations"`
	CostTrend        string         `json:"cost_trend"`
	Recommendations  []string       `json:"recommendations"`
	BudgetSuggestion *BudgetProfile `json:"budget_suggestion,omitempty"`
}

func (m *Module) improveTool() registry.ToolDefinition {
	desc := "Analyze accumulated improvement notes across R&D cycles to identify " +
		"common failure patterns, cost trends, and wasted iterations. Returns " +
		"recommendations and optionally a budget profile suggestion. " +
		"Best used every 10 cycles to calibrate the autonomous loop."

	td := handler.TypedHandler[ImproveInput, ImproveOutput](
		"rdcycle_improve",
		desc,
		m.handleImprove,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	return td
}

func (m *Module) handleImprove(_ context.Context, input ImproveInput) (ImproveOutput, error) {
	notesPath := input.NotesPath
	if notesPath == "" {
		notesPath = filepath.Join("rdcycle", "notes", "improvement_log.json")
	}

	notes, err := LoadNotes(notesPath)
	if err != nil {
		return ImproveOutput{}, fmt.Errorf("load notes: %w", err)
	}
	if len(notes) == 0 {
		return ImproveOutput{
			CyclesAnalyzed:  0,
			CommonFailures:  []string{},
			Recommendations: []string{"No improvement notes found. Run at least one cycle with rdcycle_notes first."},
		}, nil
	}

	// Analyze failure patterns.
	failureCounts := make(map[string]int)
	totalWasted := 0
	totalCost := 0.0
	for _, n := range notes {
		for _, f := range n.WhatFailed {
			failureCounts[f]++
		}
		totalWasted += n.WastedIters
		totalCost += n.TotalCost
	}

	// Find common failures (appearing in >25% of cycles).
	threshold := max(1, len(notes)/4)
	var commonFailures []string
	for failure, count := range failureCounts {
		if count >= threshold {
			commonFailures = append(commonFailures, fmt.Sprintf("%s (%d/%d cycles)", failure, count, len(notes)))
		}
	}

	avgWasted := float64(totalWasted) / float64(len(notes))

	// Determine cost trend from last 3 cycles.
	costTrend := analyzeCostTrend(notes)

	// Build recommendations.
	var recs []string
	if avgWasted > 5 {
		recs = append(recs, fmt.Sprintf("High average wasted iterations (%.1f). Consider tighter task descriptions or more specific tool prompts.", avgWasted))
	}
	if costTrend == "increasing" {
		recs = append(recs, "Cost trend is increasing. Consider switching to cheaper models for scan/verify tasks or reducing max iterations.")
	}
	if len(commonFailures) > 0 {
		recs = append(recs, fmt.Sprintf("Recurring failures detected in %d patterns. Address root causes before next cycle.", len(commonFailures)))
	}
	if avgWasted <= 2 && costTrend == "decreasing" {
		recs = append(recs, "Loop is well-calibrated. Consider increasing iteration budget for more ambitious tasks.")
	}
	if len(recs) == 0 {
		recs = append(recs, "No specific recommendations. Loop performance is stable.")
	}

	// Suggest budget adjustment if cost trend warrants it.
	var budgetSugg *BudgetProfile
	if costTrend == "increasing" && len(notes) >= 5 {
		avgCost := totalCost / float64(len(notes))
		p := PersonalProfile()
		p.DollarBudget = math.Ceil(avgCost*1.5*100) / 100 // 1.5x avg, rounded up to cents
		if p.DollarBudget < 1.0 {
			p.DollarBudget = 1.0
		}
		budgetSugg = &p
	}

	return ImproveOutput{
		CyclesAnalyzed:   len(notes),
		CommonFailures:   commonFailures,
		AvgWastedIters:   avgWasted,
		CostTrend:        costTrend,
		Recommendations:  recs,
		BudgetSuggestion: budgetSugg,
	}, nil
}

// analyzeCostTrend examines the last 3 notes to determine if costs are
// increasing, decreasing, or stable.
func analyzeCostTrend(notes []ImprovementNote) string {
	if len(notes) < 2 {
		return "stable"
	}

	// Look at last 3 (or fewer) cycles.
	start := max(len(notes)-3, 0)
	recent := notes[start:]

	increasing := 0
	decreasing := 0
	for i := 1; i < len(recent); i++ {
		diff := recent[i].TotalCost - recent[i-1].TotalCost
		if diff > 0.01 {
			increasing++
		} else if diff < -0.01 {
			decreasing++
		}
	}

	if increasing > decreasing {
		return "increasing"
	}
	if decreasing > increasing {
		return "decreasing"
	}
	return "stable"
}
