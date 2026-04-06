package rdcycle

import (
	"math"
	"testing"
)

func TestConsecutiveSuccesses_Empty(t *testing.T) {
	t.Parallel()
	if got := ConsecutiveSuccesses(nil); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestConsecutiveSuccesses_AllSuccess(t *testing.T) {
	t.Parallel()
	notes := []ImprovementNote{
		{CycleID: "c1"},
		{CycleID: "c2"},
		{CycleID: "c3"},
	}
	if got := ConsecutiveSuccesses(notes); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestConsecutiveSuccesses_TrailingFailure(t *testing.T) {
	t.Parallel()
	notes := []ImprovementNote{
		{CycleID: "c1"},
		{CycleID: "c2"},
		{CycleID: "c3", WhatFailed: []string{"broke"}},
	}
	if got := ConsecutiveSuccesses(notes); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestConsecutiveSuccesses_MixedHistory(t *testing.T) {
	t.Parallel()
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"err"}},
		{CycleID: "c2"},
		{CycleID: "c3"},
		{CycleID: "c4", WhatFailed: []string{"err"}},
		{CycleID: "c5"},
		{CycleID: "c6"},
	}
	if got := ConsecutiveSuccesses(notes); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestBudgetPct_Unlimited(t *testing.T) {
	t.Parallel()
	// Zero budget = unlimited = 1.0
	if got := BudgetPct(0, 50); got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestBudgetPct_NegativeBudget(t *testing.T) {
	t.Parallel()
	if got := BudgetPct(-10, 5); got != 1.0 {
		t.Errorf("expected 1.0 for negative budget, got %f", got)
	}
}

func TestBudgetPct_FullBudget(t *testing.T) {
	t.Parallel()
	if got := BudgetPct(100, 0); got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestBudgetPct_HalfSpent(t *testing.T) {
	t.Parallel()
	if got := BudgetPct(100, 50); math.Abs(got-0.5) > 0.001 {
		t.Errorf("expected 0.5, got %f", got)
	}
}

func TestBudgetPct_Exhausted(t *testing.T) {
	t.Parallel()
	if got := BudgetPct(100, 100); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestBudgetPct_Overspent(t *testing.T) {
	t.Parallel()
	if got := BudgetPct(100, 150); got != 0.0 {
		t.Errorf("expected 0.0 for overspent, got %f", got)
	}
}

func TestBudgetPct_SmallFraction(t *testing.T) {
	t.Parallel()
	got := BudgetPct(100, 95)
	if math.Abs(got-0.05) > 0.001 {
		t.Errorf("expected 0.05, got %f", got)
	}
}

func TestSelectStrategy_DefaultFull(t *testing.T) {
	s := SelectStrategy(nil, 0, 1.0)
	if s != StrategyFull {
		t.Fatalf("expected full, got %s", s)
	}
}

func TestSelectStrategy_LowBudgetEcosystem(t *testing.T) {
	s := SelectStrategy(nil, 0, 0.05)
	if s != StrategyEcosystem {
		t.Fatalf("expected ecosystem, got %s", s)
	}
}

func TestSelectStrategy_MetaImproveEvery10(t *testing.T) {
	notes := make([]ImprovementNote, 10)
	s := SelectStrategy(notes, 0, 0.5)
	if s != StrategyMetaImprove {
		t.Fatalf("expected meta_improve at 10 notes, got %s", s)
	}
}

func TestSelectStrategy_RecoveryOnRecentFailures(t *testing.T) {
	notes := []ImprovementNote{
		{CycleID: "c1"},
		{CycleID: "c2", WhatFailed: []string{"broken build"}},
		{CycleID: "c3", WhatFailed: []string{"test timeout"}},
	}
	s := SelectStrategy(notes, 0, 0.5)
	if s != StrategyRecovery {
		t.Fatalf("expected recovery, got %s", s)
	}
}

func TestSelectStrategy_NoRecoveryWithOneFailure(t *testing.T) {
	notes := []ImprovementNote{
		{CycleID: "c1"},
		{CycleID: "c2"},
		{CycleID: "c3", WhatFailed: []string{"minor issue"}},
	}
	s := SelectStrategy(notes, 0, 0.5)
	if s != StrategyFull {
		t.Fatalf("expected full with only 1 recent failure, got %s", s)
	}
}

func TestSelectStrategy_MaintenanceAfterSuccessStreak(t *testing.T) {
	s := SelectStrategy(nil, 5, 0.25)
	if s != StrategyMaintenance {
		t.Fatalf("expected maintenance, got %s", s)
	}
}

func TestSelectStrategy_LowBudgetTakesPriority(t *testing.T) {
	notes := make([]ImprovementNote, 10)
	s := SelectStrategy(notes, 0, 0.05)
	if s != StrategyEcosystem {
		t.Fatalf("expected ecosystem (low budget priority), got %s", s)
	}
}

func TestSelectStrategy_TwoNotesWithFailures(t *testing.T) {
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"error"}},
		{CycleID: "c2", WhatFailed: []string{"error"}},
	}
	s := SelectStrategy(notes, 0, 0.5)
	if s != StrategyRecovery {
		t.Fatalf("expected recovery with 2 failing notes, got %s", s)
	}
}
