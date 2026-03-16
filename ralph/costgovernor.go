package ralph

import (
	"fmt"
	"sync"
)

// CostVerdict is the outcome of a CostGovernor.Check() call.
type CostVerdict struct {
	// Action is one of "ok", "downgrade", or "halt".
	Action string
	// Warning is a human-readable explanation when Action is not "ok".
	Warning string
}

// CostGovernorConfig holds tuning parameters for the three-layer cost defense.
type CostGovernorConfig struct {
	// HardBudgetTokens is the absolute token ceiling. When total tokens used
	// exceeds this value, Check returns a "halt" verdict.
	// 0 means no hard limit.
	HardBudgetTokens int64
	// VelocityWindow is the number of recent iterations considered for the
	// velocity alarm. Default 5.
	VelocityWindow int
	// VelocityAlarmRate is the fraction of VelocityWindow iterations that must
	// be unproductive to trigger a "downgrade" verdict. E.g. 0.6 means 60%.
	// 0 disables velocity checking.
	VelocityAlarmRate float64
	// UnproductiveMax is the number of consecutive unproductive iterations
	// before Check returns a "halt" verdict. Default 3.
	UnproductiveMax int
}

// DefaultCostGovernorConfig returns a CostGovernorConfig with sensible defaults.
func DefaultCostGovernorConfig() CostGovernorConfig {
	return CostGovernorConfig{
		HardBudgetTokens:  0,
		VelocityWindow:    5,
		VelocityAlarmRate: 0,
		UnproductiveMax:   3,
	}
}

// iterationRecord stores per-iteration cost accounting data.
type iterationRecord struct {
	tokens     int64
	productive bool
}

// CostGovernor is a 3-layer cost defense that monitors token spend and
// productivity to decide whether to continue, downgrade, or halt execution.
type CostGovernor struct {
	mu                sync.Mutex
	config            CostGovernorConfig
	totalTokens       int64
	history           []iterationRecord
	unproductiveStreak int
}

// NewCostGovernor creates a new CostGovernor. Zero values in cfg are replaced
// with defaults for VelocityWindow and UnproductiveMax.
func NewCostGovernor(cfg CostGovernorConfig) *CostGovernor {
	if cfg.VelocityWindow <= 0 {
		cfg.VelocityWindow = 5
	}
	if cfg.UnproductiveMax <= 0 {
		cfg.UnproductiveMax = 3
	}
	return &CostGovernor{config: cfg}
}

// RecordIteration records one iteration's token usage and productivity.
// productive should be true when meaningful work was done (e.g. a task was completed or a tool call succeeded).
func (cg *CostGovernor) RecordIteration(tokens int64, productive bool) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.totalTokens += tokens
	cg.history = append(cg.history, iterationRecord{tokens: tokens, productive: productive})
	if productive {
		cg.unproductiveStreak = 0
	} else {
		cg.unproductiveStreak++
	}
}

// Check evaluates the current cost state and returns a verdict.
// Layer 1: hard budget — halt when total tokens exceed HardBudgetTokens (if set).
// Layer 2: unproductive streak — halt when consecutive unproductive iterations exceed UnproductiveMax.
// Layer 3: velocity alarm — downgrade when the recent window has too many unproductive iterations.
func (cg *CostGovernor) Check() CostVerdict {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	// Layer 1: hard budget.
	if cg.config.HardBudgetTokens > 0 && cg.totalTokens >= cg.config.HardBudgetTokens {
		return CostVerdict{
			Action:  "halt",
			Warning: formatHaltBudget(cg.totalTokens, cg.config.HardBudgetTokens),
		}
	}

	// Layer 2: unproductive streak.
	if cg.unproductiveStreak >= cg.config.UnproductiveMax {
		return CostVerdict{
			Action:  "halt",
			Warning: formatHaltStreak(cg.unproductiveStreak),
		}
	}

	// Layer 3: velocity alarm.
	if cg.config.VelocityAlarmRate > 0 && len(cg.history) >= cg.config.VelocityWindow {
		window := cg.history[len(cg.history)-cg.config.VelocityWindow:]
		unproductive := 0
		for _, r := range window {
			if !r.productive {
				unproductive++
			}
		}
		rate := float64(unproductive) / float64(cg.config.VelocityWindow)
		if rate >= cg.config.VelocityAlarmRate {
			return CostVerdict{
				Action:  "downgrade",
				Warning: formatDowngradeVelocity(unproductive, cg.config.VelocityWindow, rate),
			}
		}
	}

	return CostVerdict{Action: "ok"}
}

// TotalTokens returns the cumulative token count across all recorded iterations.
func (cg *CostGovernor) TotalTokens() int64 {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	return cg.totalTokens
}

// UnproductiveStreak returns the current count of consecutive unproductive iterations.
func (cg *CostGovernor) UnproductiveStreak() int {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	return cg.unproductiveStreak
}

func formatHaltBudget(used, limit int64) string {
	return fmt.Sprintf("hard token budget exceeded: used %d of %d", used, limit)
}

func formatHaltStreak(streak int) string {
	return fmt.Sprintf("unproductive streak of %d iterations; halting to avoid waste", streak)
}

func formatDowngradeVelocity(unproductive, window int, rate float64) string {
	return fmt.Sprintf("velocity alarm: %d/%d recent iterations unproductive (%d%%)",
		unproductive, window, int(rate*100))
}
