package rdcycle

import (
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
)

func TestCostAdapter_NilTracker(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(10000, 50)
	if adj := ca.Check(nil, 10); adj != nil {
		t.Errorf("expected nil for nil tracker, got %+v", adj)
	}
}

func TestCostAdapter_ZeroBudget(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(0, 50)
	tracker := finops.NewTracker()
	if adj := ca.Check(tracker, 10); adj != nil {
		t.Errorf("expected nil for zero budget, got %+v", adj)
	}
}

func TestCostAdapter_OnTrack(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	// Use 10% at iteration 10 (10% progress) — on track.
	tracker.Record(finops.UsageEntry{
		ToolName:     "test",
		InputTokens:  5000,
		OutputTokens: 5000,
		Timestamp:    time.Now(),
	})
	if adj := ca.Check(tracker, 10); adj != nil {
		t.Errorf("expected nil when on track, got %+v", adj)
	}
}

func TestCostAdapter_AheadOfPace(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	// Use 30% at iteration 10 (10% progress) — 3x over pace.
	tracker.Record(finops.UsageEntry{
		ToolName:     "test",
		InputTokens:  15000,
		OutputTokens: 15000,
		Timestamp:    time.Now(),
	})
	adj := ca.Check(tracker, 10)
	if adj == nil {
		t.Fatal("expected adjustment when ahead of pace")
	}
	if adj.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", adj.MaxTokens)
	}
	if adj.ModelHint != "claude-haiku-4-5-20251001" {
		t.Errorf("ModelHint = %q, want haiku", adj.ModelHint)
	}
	if adj.Warning == "" {
		t.Error("expected non-empty warning")
	}
}

func TestCostAdapter_SlightlyAhead(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	// Use 20% at iteration 10 (10% progress) — 2x over, but only 1.5x triggers.
	// 20/10 = 2.0 > 1.5 → should trigger.
	tracker.Record(finops.UsageEntry{
		ToolName:     "test",
		InputTokens:  10000,
		OutputTokens: 10000,
		Timestamp:    time.Now(),
	})
	adj := ca.Check(tracker, 10)
	if adj == nil {
		t.Fatal("expected adjustment at 2x pace")
	}
	// At exactly 2x, model hint should trigger (>2.0x threshold).
	// 20%/(10%*2.0) = 1.0, so NOT > 2.0 → no model hint.
	if adj.ModelHint != "" {
		t.Errorf("ModelHint should be empty at exactly 2x, got %q", adj.ModelHint)
	}
}

func TestCostAdapter_ZeroIteration(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	if adj := ca.Check(tracker, 0); adj != nil {
		t.Errorf("expected nil for iteration 0, got %+v", adj)
	}
}

func TestCostAdapter_LastIteration(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	tracker.Record(finops.UsageEntry{
		ToolName:     "test",
		InputTokens:  45000,
		OutputTokens: 45000,
		Timestamp:    time.Now(),
	})
	// At iteration 100 (100% progress), 90% used — on track.
	if adj := ca.Check(tracker, 100); adj != nil {
		t.Errorf("expected nil at final iteration, got %+v", adj)
	}
}

func TestCombineSelectors_NilAdapter(t *testing.T) {
	t.Parallel()
	baseCalled := false
	base := func(iter int, ids []string) string {
		baseCalled = true
		return "sonnet"
	}
	combined := CombineSelectors(base, nil, nil)
	result := combined(1, nil)
	if !baseCalled {
		t.Error("expected base selector to be called")
	}
	if result != "sonnet" {
		t.Errorf("result = %q, want 'sonnet'", result)
	}
}

func TestCombineSelectors_AdapterOverrides(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	// 40% at 10% progress — way over pace.
	tracker.Record(finops.UsageEntry{
		ToolName:     "test",
		InputTokens:  20000,
		OutputTokens: 20000,
		Timestamp:    time.Now(),
	})
	base := func(iter int, ids []string) string { return "sonnet" }
	combined := CombineSelectors(base, ca, tracker)
	result := combined(10, nil)
	if result != "claude-haiku-4-5-20251001" {
		t.Errorf("expected haiku override, got %q", result)
	}
}

func TestCombineSelectors_NilBase(t *testing.T) {
	t.Parallel()
	ca := NewCostAdapter(100000, 100)
	tracker := finops.NewTracker()
	combined := CombineSelectors(nil, ca, tracker)
	result := combined(1, nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
