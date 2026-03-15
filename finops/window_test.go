package finops

import (
	"testing"
	"time"
)

// controllableClock provides a monotonically advancing clock for testing.
type controllableClock struct {
	current time.Time
}

func newClock(t time.Time) *controllableClock {
	return &controllableClock{current: t}
}

func (c *controllableClock) Now() time.Time {
	return c.current
}

func (c *controllableClock) Advance(d time.Duration) {
	c.current = c.current.Add(d)
}

func TestWindowedTracker_NoRotationWithinWindow(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetHourly, WithNowFunc(clock.Now))

	tracker1 := wt.Tracker()
	tracker1.Record(UsageEntry{InputTokens: 10, OutputTokens: 20})

	// Advance by 30 minutes — still within the hour window.
	clock.Advance(30 * time.Minute)
	tracker2 := wt.Tracker()

	if tracker1 != tracker2 {
		t.Error("expected the same Tracker within the window, got a different one")
	}

	if tracker2.Total() != 30 {
		t.Errorf("expected Total=30, got %d", tracker2.Total())
	}
}

func TestWindowedTracker_HourlyRotation(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetHourly, WithNowFunc(clock.Now))

	t1 := wt.Tracker()
	t1.Record(UsageEntry{InputTokens: 5, OutputTokens: 10})

	// Advance past 1 hour.
	clock.Advance(61 * time.Minute)
	t2 := wt.Tracker()

	if t1 == t2 {
		t.Error("expected a new Tracker after hourly rotation")
	}
	if t2.Total() != 0 {
		t.Errorf("expected fresh Tracker with Total=0 after rotation, got %d", t2.Total())
	}

	history := wt.History()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry after rotation, got %d", len(history))
	}
	if history[0].TotalInput != 5 {
		t.Errorf("expected history TotalInput=5, got %d", history[0].TotalInput)
	}
	if history[0].TotalOutput != 10 {
		t.Errorf("expected history TotalOutput=10, got %d", history[0].TotalOutput)
	}
}

func TestWindowedTracker_DailyRotation(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetDaily, WithNowFunc(clock.Now))

	wt.Tracker().Record(UsageEntry{InputTokens: 100, OutputTokens: 200})

	// Advance by 25 hours.
	clock.Advance(25 * time.Hour)
	fresh := wt.Tracker()

	if fresh.Total() != 0 {
		t.Errorf("expected fresh tracker after daily rotation, got Total=%d", fresh.Total())
	}
	if len(wt.History()) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(wt.History()))
	}
}

func TestWindowedTracker_HistoryAccumulation(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetHourly, WithNowFunc(clock.Now))

	// Run 3 complete windows.
	for i := 0; i < 3; i++ {
		wt.Tracker().Record(UsageEntry{InputTokens: i + 1, OutputTokens: (i + 1) * 2})
		clock.Advance(61 * time.Minute)
	}
	wt.Tracker() // trigger rotation of the last window

	history := wt.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	// Verify first window had 1 input token.
	if history[0].TotalInput != 1 {
		t.Errorf("expected history[0].TotalInput=1, got %d", history[0].TotalInput)
	}
}

func TestWindowedTracker_MaxHistory(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetHourly,
		WithNowFunc(clock.Now),
		WithMaxHistory(2),
	)

	// Create 4 completed windows — only 2 should be kept.
	for i := 0; i < 4; i++ {
		wt.Tracker().Record(UsageEntry{InputTokens: i + 1})
		clock.Advance(61 * time.Minute)
	}
	wt.Tracker() // trigger final rotation

	history := wt.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries with maxHistory=2, got %d", len(history))
	}
	// The two most recent windows: index 2 (InputTokens=3) and 3 (InputTokens=4).
	if history[0].TotalInput != 3 {
		t.Errorf("expected history[0].TotalInput=3, got %d", history[0].TotalInput)
	}
	if history[1].TotalInput != 4 {
		t.Errorf("expected history[1].TotalInput=4, got %d", history[1].TotalInput)
	}
}

func TestWindowedTracker_CurrentWindow(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetHourly, WithNowFunc(clock.Now))

	tr := wt.Tracker()
	tr.Record(UsageEntry{InputTokens: 7, OutputTokens: 13})

	clock.Advance(15 * time.Minute)
	ws := wt.CurrentWindow()

	if ws.TotalInput != 7 {
		t.Errorf("expected CurrentWindow.TotalInput=7, got %d", ws.TotalInput)
	}
	if ws.TotalOutput != 13 {
		t.Errorf("expected CurrentWindow.TotalOutput=13, got %d", ws.TotalOutput)
	}
	if ws.Start.IsZero() {
		t.Error("expected non-zero Start in CurrentWindow")
	}
}

func TestWindowedTracker_WeeklyInterval(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetWeekly, WithNowFunc(clock.Now))

	wt.Tracker().Record(UsageEntry{InputTokens: 50})

	// Advance 6 days — still within 7-day window.
	clock.Advance(6 * 24 * time.Hour)
	t1 := wt.Tracker()
	if t1.Total() == 0 {
		t.Error("expected same window after 6 days with weekly interval")
	}

	// Advance past 7 days total.
	clock.Advance(2 * 24 * time.Hour)
	t2 := wt.Tracker()
	if t2.Total() != 0 {
		t.Errorf("expected fresh tracker after 8 days with weekly interval, got Total=%d", t2.Total())
	}
}

func TestWindowedTracker_MonthlyInterval(t *testing.T) {
	t.Parallel()

	clock := newClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	wt := NewWindowedTracker(Config{}, ResetMonthly, WithNowFunc(clock.Now))

	wt.Tracker().Record(UsageEntry{InputTokens: 200})

	// Advance 29 days — still within 30-day window.
	clock.Advance(29 * 24 * time.Hour)
	if wt.Tracker().Total() == 0 {
		t.Error("expected same window after 29 days with monthly interval")
	}

	// Advance past 30 days total.
	clock.Advance(2 * 24 * time.Hour)
	if wt.Tracker().Total() != 0 {
		t.Errorf("expected fresh tracker after 31 days with monthly interval")
	}
}
