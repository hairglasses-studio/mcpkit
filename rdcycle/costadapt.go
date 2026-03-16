package rdcycle

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/finops"
)

// CostAdjustment contains recommended changes when cost is running ahead of pace.
type CostAdjustment struct {
	// MaxTokens is the recommended max tokens per request (0 = no change).
	MaxTokens int
	// ModelHint is a suggested cheaper model (empty = no change).
	ModelHint string
	// Warning is a human-readable warning message.
	Warning string
}

// CostAdapter monitors budget consumption rate vs iteration pace and recommends
// adjustments when cost is running ahead of schedule.
type CostAdapter struct {
	// TotalBudget is the total token budget for the run.
	TotalBudget int64
	// MaxIterations is the expected total iterations.
	MaxIterations int
	// FallbackModel is the cheaper model to suggest when over budget pace.
	FallbackModel string
	// ReducedMaxTokens is the reduced token limit to suggest.
	ReducedMaxTokens int
}

// NewCostAdapter creates a CostAdapter with the given budget parameters.
func NewCostAdapter(totalBudget int64, maxIterations int) *CostAdapter {
	return &CostAdapter{
		TotalBudget:      totalBudget,
		MaxIterations:    maxIterations,
		FallbackModel:    "claude-haiku-4-5-20251001",
		ReducedMaxTokens: 1024,
	}
}

// Check examines current cost vs expected pace and returns an adjustment if needed.
// Returns nil when cost is on track.
func (ca *CostAdapter) Check(tracker *finops.Tracker, currentIteration int) *CostAdjustment {
	if ca.TotalBudget <= 0 || ca.MaxIterations <= 0 || tracker == nil {
		return nil
	}
	if currentIteration <= 0 {
		return nil
	}

	used := tracker.Total()
	if used <= 0 {
		return nil
	}

	// Calculate budget pace: what fraction of budget should be used at this point.
	expectedFraction := float64(currentIteration) / float64(ca.MaxIterations)
	actualFraction := float64(used) / float64(ca.TotalBudget)

	// If actual usage is more than 1.5x the expected fraction, recommend adjustment.
	if actualFraction <= expectedFraction*1.5 {
		return nil
	}

	remaining := ca.TotalBudget - used
	remainingIters := ca.MaxIterations - currentIteration
	if remainingIters <= 0 {
		remainingIters = 1
	}
	perIterBudget := remaining / int64(remainingIters)

	adj := &CostAdjustment{
		Warning: fmt.Sprintf(
			"Cost running ahead of pace: %.1f%% budget used at %.1f%% progress (iteration %d/%d, %d tokens remaining for %d iterations, ~%d per iter)",
			actualFraction*100, expectedFraction*100,
			currentIteration, ca.MaxIterations,
			remaining, remainingIters, perIterBudget),
	}

	// Suggest reduced max tokens if per-iter budget is less than current limit.
	if ca.ReducedMaxTokens > 0 {
		adj.MaxTokens = ca.ReducedMaxTokens
	}

	// Suggest cheaper model if usage is more than 2x expected.
	if actualFraction > expectedFraction*2.0 && ca.FallbackModel != "" {
		adj.ModelHint = ca.FallbackModel
	}

	return adj
}

// CombineSelectors creates a ModelSelector that uses the CostAdapter's recommendation
// when cost is ahead of pace, falling back to the base selector otherwise.
func CombineSelectors(base func(int, []string) string, adapter *CostAdapter, tracker *finops.Tracker) func(int, []string) string {
	if adapter == nil || tracker == nil {
		return base
	}
	return func(iteration int, completedIDs []string) string {
		if adj := adapter.Check(tracker, iteration); adj != nil && adj.ModelHint != "" {
			return adj.ModelHint
		}
		if base != nil {
			return base(iteration, completedIDs)
		}
		return ""
	}
}
