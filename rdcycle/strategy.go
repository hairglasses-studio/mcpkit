package rdcycle

// CycleStrategy determines which task sources and phases are active in a cycle.
type CycleStrategy string

const (
	// StrategyFull activates all phases: scan, plan, implement, verify, reflect, report, schedule.
	StrategyFull CycleStrategy = "full"
	// StrategyMaintenance skips implementation: verify, reflect, schedule only.
	StrategyMaintenance CycleStrategy = "maintenance"
	// StrategyRecovery focuses on fixing: verify, fix, verify, schedule.
	StrategyRecovery CycleStrategy = "recovery"
	// StrategyMetaImprove runs only self-improvement analysis.
	StrategyMetaImprove CycleStrategy = "meta_improve"
	// StrategyEcosystem runs scan, plan, schedule (no implementation).
	StrategyEcosystem CycleStrategy = "ecosystem"
)

// ConsecutiveSuccesses counts the trailing streak of improvement notes
// that have no failures (WhatFailed is empty). This is used by callers
// to compute the consecutiveSuccess parameter for SelectStrategy.
func ConsecutiveSuccesses(notes []ImprovementNote) int {
	streak := 0
	for i := len(notes) - 1; i >= 0; i-- {
		if len(notes[i].WhatFailed) > 0 {
			break
		}
		streak++
	}
	return streak
}

// BudgetPct computes the fraction of budget remaining as a value in [0.0, 1.0].
// totalBudget is the total dollar budget; spent is the amount already consumed.
// Returns 1.0 if totalBudget is zero or negative (meaning unlimited).
func BudgetPct(totalBudget, spent float64) float64 {
	if totalBudget <= 0 {
		return 1.0
	}
	remaining := totalBudget - spent
	if remaining <= 0 {
		return 0.0
	}
	pct := remaining / totalBudget
	if pct > 1.0 {
		return 1.0
	}
	return pct
}

// SelectStrategy chooses the optimal cycle strategy based on history and budget.
func SelectStrategy(notes []ImprovementNote, consecutiveSuccess int, budgetPct float64) CycleStrategy {
	if budgetPct < 0.1 {
		return StrategyEcosystem
	}

	if len(notes) > 0 && len(notes)%10 == 0 {
		return StrategyMetaImprove
	}

	if len(notes) >= 2 {
		recentFailures := 0
		start := max(len(notes)-3, 0)
		for _, n := range notes[start:] {
			if len(n.WhatFailed) > 0 {
				recentFailures++
			}
		}
		if recentFailures >= 2 {
			return StrategyRecovery
		}
	}

	if consecutiveSuccess >= 5 && budgetPct < 0.3 {
		return StrategyMaintenance
	}

	return StrategyFull
}
