package rdcycle

import "testing"

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
