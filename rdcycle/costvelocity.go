package rdcycle

import (
	"fmt"
	"sync"
)

// CycleRecord stores the cost and progress of a single R&D cycle.
type CycleRecord struct {
	CycleNum   int     `json:"cycle_num"`
	Cost       float64 `json:"cost"`
	Productive bool    `json:"productive"` // true if the cycle made meaningful progress
}

// VelocityAlarm describes why the governor is raising an alarm.
type VelocityAlarm struct {
	Reason     string  `json:"reason"`
	AvgCost    float64 `json:"avg_cost"`
	WindowSize int     `json:"window_size"`
}

// CostVelocityGovernor monitors cross-cycle budget consumption and detects
// cost velocity spikes, unproductive streaks, and budget exhaustion.
// It complements the per-loop CostAdapter with cross-cycle awareness.
type CostVelocityGovernor struct {
	mu              sync.Mutex
	WindowSize      int     // rolling window size (default 5)
	AlarmPerCycle   float64 // max $/cycle average before alarm (default 5.0)
	UnproductiveCap int     // halt after N consecutive unproductive cycles (default 3)
	history         []CycleRecord
}

// NewCostVelocityGovernor creates a governor with the given thresholds.
func NewCostVelocityGovernor(windowSize int, alarmPerCycle float64, unproductiveCap int) *CostVelocityGovernor {
	if windowSize <= 0 {
		windowSize = 5
	}
	if alarmPerCycle <= 0 {
		alarmPerCycle = 5.0
	}
	if unproductiveCap <= 0 {
		unproductiveCap = 3
	}
	return &CostVelocityGovernor{
		WindowSize:      windowSize,
		AlarmPerCycle:   alarmPerCycle,
		UnproductiveCap: unproductiveCap,
	}
}

// Record adds a cycle record to the history.
func (g *CostVelocityGovernor) Record(record CycleRecord) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.history = append(g.history, record)
}

// Check examines the recent history for cost velocity issues.
// Returns nil if everything is within bounds.
func (g *CostVelocityGovernor) Check() *VelocityAlarm {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.history) == 0 {
		return nil
	}

	// Check rolling average cost.
	windowStart := max(len(g.history)-g.WindowSize, 0)
	window := g.history[windowStart:]

	totalCost := 0.0
	for _, r := range window {
		totalCost += r.Cost
	}
	avgCost := totalCost / float64(len(window))

	if avgCost > g.AlarmPerCycle {
		return &VelocityAlarm{
			Reason:     fmt.Sprintf("rolling average cost $%.2f exceeds alarm threshold $%.2f", avgCost, g.AlarmPerCycle),
			AvgCost:    avgCost,
			WindowSize: len(window),
		}
	}

	return nil
}

// ShouldHalt returns true if the governor recommends stopping perpetual execution.
// This happens when unproductive cycles exceed the cap or cost alarm is active.
func (g *CostVelocityGovernor) ShouldHalt() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.history) == 0 {
		return false
	}

	// Check unproductive streak from the end.
	streak := 0
	for i := len(g.history) - 1; i >= 0; i-- {
		if !g.history[i].Productive {
			streak++
		} else {
			break
		}
	}
	if streak >= g.UnproductiveCap {
		return true
	}

	// Check cost velocity.
	windowStart := max(len(g.history)-g.WindowSize, 0)
	window := g.history[windowStart:]

	totalCost := 0.0
	for _, r := range window {
		totalCost += r.Cost
	}
	avgCost := totalCost / float64(len(window))

	return avgCost > g.AlarmPerCycle
}

// TotalCost returns the sum of all recorded cycle costs.
func (g *CostVelocityGovernor) TotalCost() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	total := 0.0
	for _, r := range g.history {
		total += r.Cost
	}
	return total
}

// CycleCount returns the number of recorded cycles.
func (g *CostVelocityGovernor) CycleCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.history)
}
