package finops

import "sync"

// Tracker records and queries token usage. Thread-safe.
type Tracker struct {
	mu          sync.RWMutex
	config      Config
	entries     []UsageEntry
	totalTokens int64
}

// NewTracker creates a new Tracker with optional configuration.
func NewTracker(config ...Config) *Tracker {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	return &Tracker{config: cfg}
}

// Record adds a usage entry to the tracker.
func (t *Tracker) Record(entry UsageEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, entry)
	t.totalTokens += int64(entry.InputTokens) + int64(entry.OutputTokens)
}

// Summary returns an aggregated view of all recorded usage.
func (t *Tracker) Summary() UsageSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := UsageSummary{
		ByTool:     make(map[string]int64),
		ByCategory: make(map[string]int64),
	}
	for _, e := range t.entries {
		tokens := int64(e.InputTokens) + int64(e.OutputTokens)
		s.TotalInputTokens += int64(e.InputTokens)
		s.TotalOutputTokens += int64(e.OutputTokens)
		s.TotalInvocations++
		s.ByTool[e.ToolName] += tokens
		s.ByCategory[e.Category] += tokens
	}
	return s
}

// Reset clears all recorded usage data.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = nil
	t.totalTokens = 0
}

// Total returns the total tokens recorded so far.
func (t *Tracker) Total() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens
}
