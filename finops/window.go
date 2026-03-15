package finops

import (
	"sync"
	"time"
)

// ResetInterval defines how often a windowed tracker rotates.
type ResetInterval int

const (
	// ResetHourly rotates the tracker every hour.
	ResetHourly ResetInterval = iota
	// ResetDaily rotates the tracker every 24 hours.
	ResetDaily
	// ResetWeekly rotates the tracker every 7 days.
	ResetWeekly
	// ResetMonthly rotates the tracker every 30 days.
	ResetMonthly
)

// WindowSummary is a snapshot of usage for a completed time window.
type WindowSummary struct {
	Start       time.Time
	End         time.Time
	TotalInput  int
	TotalOutput int
	TotalCost   float64
}

// WindowedTracker provides lazy-rotating time-windowed token tracking.
// Rotation happens on the next call to Tracker() after the current window expires.
// Thread-safe.
type WindowedTracker struct {
	mu          sync.RWMutex
	config      Config
	interval    ResetInterval
	current     *Tracker
	windowStart time.Time
	history     []WindowSummary
	nowFunc     func() time.Time
	maxHistory  int
}

// WindowOption configures a WindowedTracker.
type WindowOption func(*WindowedTracker)

// WithMaxHistory sets the maximum number of historical window summaries to retain.
// Older entries are dropped when the limit is exceeded. Default is 0 (unlimited).
func WithMaxHistory(n int) WindowOption {
	return func(wt *WindowedTracker) {
		wt.maxHistory = n
	}
}

// WithNowFunc overrides the clock used for window boundary calculations.
// Intended for testing only.
func WithNowFunc(f func() time.Time) WindowOption {
	return func(wt *WindowedTracker) {
		wt.nowFunc = f
	}
}

// NewWindowedTracker creates a new WindowedTracker with the given config and reset interval.
func NewWindowedTracker(config Config, interval ResetInterval, opts ...WindowOption) *WindowedTracker {
	wt := &WindowedTracker{
		config:   config,
		interval: interval,
		nowFunc:  time.Now,
	}
	for _, opt := range opts {
		opt(wt)
	}
	wt.windowStart = wt.nowFunc()
	wt.current = NewTracker(config)
	return wt
}

// windowDuration returns the duration of the interval.
func (wt *WindowedTracker) windowDuration() time.Duration {
	switch wt.interval {
	case ResetHourly:
		return time.Hour
	case ResetDaily:
		return 24 * time.Hour
	case ResetWeekly:
		return 7 * 24 * time.Hour
	case ResetMonthly:
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// rotate snapshots the current window to history and starts a fresh tracker.
// Caller must hold wt.mu (write lock).
func (wt *WindowedTracker) rotate(now time.Time) {
	summary := wt.current.Summary()
	ws := WindowSummary{
		Start:       wt.windowStart,
		End:         now,
		TotalInput:  int(summary.TotalInputTokens),
		TotalOutput: int(summary.TotalOutputTokens),
	}
	// Include cost if a CostPolicy is attached.
	if wt.config.CostPolicy != nil {
		ws.TotalCost = wt.config.CostPolicy.TotalCost()
	}
	wt.history = append(wt.history, ws)

	// Trim history if a max is configured.
	if wt.maxHistory > 0 && len(wt.history) > wt.maxHistory {
		wt.history = wt.history[len(wt.history)-wt.maxHistory:]
	}

	wt.windowStart = now
	wt.current = NewTracker(wt.config)
}

// Tracker returns the current window's Tracker, rotating if the window has expired.
func (wt *WindowedTracker) Tracker() *Tracker {
	now := wt.nowFunc()
	wt.mu.Lock()
	defer wt.mu.Unlock()

	if now.Sub(wt.windowStart) >= wt.windowDuration() {
		wt.rotate(now)
	}
	return wt.current
}

// History returns a copy of all completed window summaries, oldest first.
func (wt *WindowedTracker) History() []WindowSummary {
	wt.mu.RLock()
	defer wt.mu.RUnlock()
	result := make([]WindowSummary, len(wt.history))
	copy(result, wt.history)
	return result
}

// CurrentWindow returns a WindowSummary snapshot of the active (not yet rotated) window.
func (wt *WindowedTracker) CurrentWindow() WindowSummary {
	wt.mu.RLock()
	defer wt.mu.RUnlock()

	now := wt.nowFunc()
	summary := wt.current.Summary()
	ws := WindowSummary{
		Start:       wt.windowStart,
		End:         now,
		TotalInput:  int(summary.TotalInputTokens),
		TotalOutput: int(summary.TotalOutputTokens),
	}
	if wt.config.CostPolicy != nil {
		ws.TotalCost = wt.config.CostPolicy.TotalCost()
	}
	return ws
}
