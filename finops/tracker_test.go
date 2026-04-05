//go:build !official_sdk

package finops

import (
	"sync"
	"testing"
	"time"
)

// TestNewTracker_ZeroConfig verifies that NewTracker with no arguments creates a
// usable tracker with a zero-value Config.
func TestNewTracker_ZeroConfig(t *testing.T) {
	t.Parallel()

	tr := NewTracker()
	if tr == nil {
		t.Fatal("NewTracker() returned nil")
	}
	if tr.Total() != 0 {
		t.Errorf("expected Total()=0 on fresh tracker, got %d", tr.Total())
	}
	s := tr.Summary()
	if s.TotalInvocations != 0 {
		t.Errorf("expected TotalInvocations=0, got %d", s.TotalInvocations)
	}
	if s.ByTool == nil {
		t.Error("expected non-nil ByTool map")
	}
	if s.ByCategory == nil {
		t.Error("expected non-nil ByCategory map")
	}
}

// TestNewTracker_CustomConfig verifies that NewTracker stores the provided Config.
func TestNewTracker_CustomConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TokenBudget: 500,
	}
	tr := NewTracker(cfg)
	if tr == nil {
		t.Fatal("NewTracker(cfg) returned nil")
	}
	// Confirm the budget is honoured by checking the stored config value
	// indirectly: pre-populate enough tokens and verify budget trips.
	for range 10 {
		tr.Record(UsageEntry{InputTokens: 50, OutputTokens: 50})
	}
	// 10 * (50+50) = 1000 tokens recorded — exceeds the 500-token budget.
	if tr.Total() != 1000 {
		t.Errorf("expected total=1000 after seeding, got %d", tr.Total())
	}
	// config.TokenBudget should be 500
	if tr.config.TokenBudget != 500 {
		t.Errorf("expected config.TokenBudget=500, got %d", tr.config.TokenBudget)
	}
}

// TestTracker_RecordAddsEntry verifies that each Record call appends an entry
// and is reflected in Summary and Total.
func TestTracker_RecordAddsEntry(t *testing.T) {
	t.Parallel()

	tr := NewTracker()

	if tr.Total() != 0 {
		t.Fatalf("expected Total()=0 before any record, got %d", tr.Total())
	}

	tr.Record(UsageEntry{
		ToolName:     "alpha",
		Category:     "llm",
		InputTokens:  12,
		OutputTokens: 8,
	})

	if tr.Total() != 20 {
		t.Errorf("expected Total()=20 after one entry (12+8), got %d", tr.Total())
	}
	s := tr.Summary()
	if s.TotalInvocations != 1 {
		t.Errorf("expected TotalInvocations=1, got %d", s.TotalInvocations)
	}
}

// TestTracker_SummaryAggregatesByToolAndCategory verifies ByTool and ByCategory
// map values are correctly accumulated across multiple entries.
func TestTracker_SummaryAggregatesByToolAndCategory(t *testing.T) {
	t.Parallel()

	tr := NewTracker()
	entries := []UsageEntry{
		{ToolName: "search", Category: "retrieval", InputTokens: 10, OutputTokens: 20},
		{ToolName: "search", Category: "retrieval", InputTokens: 5, OutputTokens: 10},
		{ToolName: "generate", Category: "llm", InputTokens: 30, OutputTokens: 60},
	}
	for _, e := range entries {
		tr.Record(e)
	}

	s := tr.Summary()

	// ByTool: search = (10+20)+(5+10) = 45; generate = 30+60 = 90
	if s.ByTool["search"] != 45 {
		t.Errorf("expected ByTool[search]=45, got %d", s.ByTool["search"])
	}
	if s.ByTool["generate"] != 90 {
		t.Errorf("expected ByTool[generate]=90, got %d", s.ByTool["generate"])
	}

	// ByCategory: retrieval = 45; llm = 90
	if s.ByCategory["retrieval"] != 45 {
		t.Errorf("expected ByCategory[retrieval]=45, got %d", s.ByCategory["retrieval"])
	}
	if s.ByCategory["llm"] != 90 {
		t.Errorf("expected ByCategory[llm]=90, got %d", s.ByCategory["llm"])
	}
}

// TestTracker_SummaryInputOutputSplit verifies that TotalInputTokens and
// TotalOutputTokens are tracked separately and correctly summed.
func TestTracker_SummaryInputOutputSplit(t *testing.T) {
	t.Parallel()

	tr := NewTracker()
	tr.Record(UsageEntry{ToolName: "t1", Category: "c", InputTokens: 100, OutputTokens: 200})
	tr.Record(UsageEntry{ToolName: "t2", Category: "c", InputTokens: 50, OutputTokens: 75})

	s := tr.Summary()

	if s.TotalInputTokens != 150 {
		t.Errorf("expected TotalInputTokens=150, got %d", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 275 {
		t.Errorf("expected TotalOutputTokens=275, got %d", s.TotalOutputTokens)
	}
	// Total() must equal input+output across all entries.
	expectedTotal := int64(150 + 275)
	if tr.Total() != expectedTotal {
		t.Errorf("expected Total()=%d, got %d", expectedTotal, tr.Total())
	}
}

// TestTracker_TotalRunningAccumulation verifies that Total increases monotonically
// with each Record call.
func TestTracker_TotalRunningAccumulation(t *testing.T) {
	t.Parallel()

	tr := NewTracker()

	steps := []struct {
		in, out   int
		wantTotal int64
	}{
		{10, 10, 20},
		{5, 15, 40},
		{0, 100, 140},
	}

	for i, step := range steps {
		tr.Record(UsageEntry{InputTokens: step.in, OutputTokens: step.out})
		if tr.Total() != step.wantTotal {
			t.Errorf("step %d: expected Total()=%d, got %d", i+1, step.wantTotal, tr.Total())
		}
	}
}

// TestTracker_ResetClearsData verifies that Reset returns Total to zero and
// produces an empty Summary.
func TestTracker_ResetClearsData(t *testing.T) {
	t.Parallel()

	tr := NewTracker()
	tr.Record(UsageEntry{ToolName: "x", Category: "y", InputTokens: 999, OutputTokens: 1})

	if tr.Total() == 0 {
		t.Fatal("pre-condition: expected non-zero Total before Reset")
	}

	tr.Reset()

	if tr.Total() != 0 {
		t.Errorf("expected Total()=0 after Reset, got %d", tr.Total())
	}

	s := tr.Summary()
	if s.TotalInvocations != 0 {
		t.Errorf("expected TotalInvocations=0 after Reset, got %d", s.TotalInvocations)
	}
	if s.TotalInputTokens != 0 {
		t.Errorf("expected TotalInputTokens=0 after Reset, got %d", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 0 {
		t.Errorf("expected TotalOutputTokens=0 after Reset, got %d", s.TotalOutputTokens)
	}
	if len(s.ByTool) != 0 {
		t.Errorf("expected empty ByTool after Reset, got %v", s.ByTool)
	}
	if len(s.ByCategory) != 0 {
		t.Errorf("expected empty ByCategory after Reset, got %v", s.ByCategory)
	}
}

// TestTracker_ConcurrentRecord verifies that concurrent Record calls from
// multiple goroutines do not cause data races and that all entries are counted.
func TestTracker_ConcurrentRecord(t *testing.T) {
	t.Parallel()

	const goroutines = 20
	const recordsPerGoroutine = 50

	tr := NewTracker()
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range recordsPerGoroutine {
				tr.Record(UsageEntry{
					ToolName:     "concurrent-tool",
					Category:     "stress",
					InputTokens:  1,
					OutputTokens: 1,
					Timestamp:    time.Now(),
				})
			}
		}()
	}
	wg.Wait()

	expectedInvocations := int64(goroutines * recordsPerGoroutine)
	s := tr.Summary()
	if s.TotalInvocations != expectedInvocations {
		t.Errorf("expected TotalInvocations=%d after concurrent writes, got %d",
			expectedInvocations, s.TotalInvocations)
	}

	// Each entry contributes 2 tokens (1 in + 1 out).
	expectedTotal := expectedInvocations * 2
	if tr.Total() != expectedTotal {
		t.Errorf("expected Total()=%d, got %d", expectedTotal, tr.Total())
	}
}

// TestTracker_SummaryAfterReset verifies that recording after a Reset starts
// fresh accumulation.
func TestTracker_SummaryAfterReset(t *testing.T) {
	t.Parallel()

	tr := NewTracker()
	tr.Record(UsageEntry{ToolName: "old", Category: "a", InputTokens: 500, OutputTokens: 500})
	tr.Reset()

	tr.Record(UsageEntry{ToolName: "new", Category: "b", InputTokens: 3, OutputTokens: 7})

	s := tr.Summary()
	if s.TotalInvocations != 1 {
		t.Errorf("expected TotalInvocations=1 after reset+record, got %d", s.TotalInvocations)
	}
	if s.TotalInputTokens != 3 {
		t.Errorf("expected TotalInputTokens=3, got %d", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 7 {
		t.Errorf("expected TotalOutputTokens=7, got %d", s.TotalOutputTokens)
	}
	if _, hadOld := s.ByTool["old"]; hadOld {
		t.Error("expected no entry for 'old' tool after reset")
	}
	if s.ByTool["new"] != 10 {
		t.Errorf("expected ByTool[new]=10, got %d", s.ByTool["new"])
	}
}
