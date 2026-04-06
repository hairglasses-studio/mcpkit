//go:build !official_sdk

package finops

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// CostTracker is the interface for tracking dollar-denominated spend.
// *CostPolicy satisfies this interface.
type CostTracker interface {
	RecordCost(cost float64) error
	TotalCost() float64
	RemainingBudget() float64
}

// BudgetAction defines what happens when a budget limit is exceeded.
type BudgetAction string

const (
	// ActionWarn logs a warning but allows the operation to proceed.
	ActionWarn BudgetAction = "warn"
	// ActionBlock prevents new operations from executing.
	ActionBlock BudgetAction = "block"
	// ActionDowngrade signals that the caller should switch to a cheaper model.
	ActionDowngrade BudgetAction = "downgrade"
)

// BudgetAlert represents a budget threshold crossing event.
type BudgetAlert struct {
	PolicyName  string
	Threshold   float64 // The threshold percentage that was crossed (e.g. 0.5, 0.75, 0.9)
	CurrentCost float64
	BudgetLimit float64
	Timestamp   time.Time
}

// BudgetPolicy defines a spending limit with alert thresholds and an enforcement action.
type BudgetPolicy struct {
	Name       string        // Human-readable policy name
	Limit      float64       // Max spend in dollars
	Window     time.Duration // Time window (e.g. 24h, 168h for weekly)
	Thresholds []float64     // Alert at these percentages (e.g. [0.5, 0.75, 0.9])
	Action     BudgetAction  // What to do when limit is exceeded
}

// BudgetEnforcer monitors cost and enforces budget limits. Thread-safe.
type BudgetEnforcer struct {
	tracker          CostTracker
	policies         []BudgetPolicy
	alertFunc        func(alert BudgetAlert)
	mu               sync.RWMutex
	firedThresholds  map[string]map[float64]bool // policyName -> threshold -> fired
	internalCost     float64                     // internal cost accumulator (used when tracker is nil)
	internalBudget   float64                     // internal budget limit
	useInternalTrack bool                        // true when no external tracker provided
}

// NewBudgetEnforcer creates a new BudgetEnforcer.
// If tracker is nil, the enforcer uses an internal cost accumulator.
// alertFunc is called whenever a threshold is crossed; it may be nil.
func NewBudgetEnforcer(tracker CostTracker, alertFunc func(BudgetAlert)) *BudgetEnforcer {
	e := &BudgetEnforcer{
		tracker:         tracker,
		alertFunc:       alertFunc,
		firedThresholds: make(map[string]map[float64]bool),
	}
	if tracker == nil {
		e.useInternalTrack = true
	}
	return e
}

// AddPolicy adds a budget policy to the enforcer.
func (e *BudgetEnforcer) AddPolicy(p BudgetPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.policies = append(e.policies, p)
	e.firedThresholds[p.Name] = make(map[float64]bool)

	// Track the most restrictive budget limit for RemainingBudget calculations.
	if e.useInternalTrack && p.Limit > e.internalBudget {
		e.internalBudget = p.Limit
	}
}

// Check evaluates all policies against current spend and returns the most
// restrictive action. Returns ActionWarn with nil error if all policies pass.
// Returns ActionBlock with a BudgetExceededError if any blocking policy is exceeded.
func (e *BudgetEnforcer) Check(ctx context.Context) (BudgetAction, error) {
	e.mu.RLock()
	currentCost := e.currentSpendLocked()
	policies := make([]BudgetPolicy, len(e.policies))
	copy(policies, e.policies)
	e.mu.RUnlock()

	// Determine most restrictive action across all policies.
	result := ActionWarn
	for _, p := range policies {
		if p.Limit <= 0 {
			continue
		}
		ratio := currentCost / p.Limit

		// Fire threshold alerts.
		e.fireAlerts(p, currentCost)

		// Check if limit is exceeded.
		if ratio >= 1.0 {
			switch p.Action {
			case ActionBlock:
				return ActionBlock, &DollarBudgetExceededError{
					Limit: p.Limit,
					Used:  currentCost,
				}
			case ActionDowngrade:
				result = ActionDowngrade
			}
			// ActionWarn: continue, just note it
		}
	}

	return result, nil
}

// RecordCost records a dollar cost amount. If using an external tracker,
// delegates to it. Otherwise uses the internal accumulator.
func (e *BudgetEnforcer) RecordCost(amount float64) {
	e.mu.Lock()
	if e.useInternalTrack {
		e.internalCost += amount
		e.mu.Unlock()
	} else {
		e.mu.Unlock()
		// Record to external tracker (which has its own lock).
		_ = e.tracker.RecordCost(amount)
	}

	// Check and fire threshold alerts after recording.
	e.mu.RLock()
	currentCost := e.currentSpendLocked()
	policies := make([]BudgetPolicy, len(e.policies))
	copy(policies, e.policies)
	e.mu.RUnlock()

	for _, p := range policies {
		e.fireAlerts(p, currentCost)
	}
}

// CurrentSpend returns the total accumulated dollar cost.
func (e *BudgetEnforcer) CurrentSpend() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentSpendLocked()
}

// RemainingBudget returns the remaining dollar budget based on the most
// restrictive policy limit. Returns 0 if no policies are configured or
// if the budget is exceeded.
func (e *BudgetEnforcer) RemainingBudget() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.policies) == 0 {
		return 0
	}

	currentCost := e.currentSpendLocked()

	// Find the smallest (most restrictive) policy limit.
	minLimit := e.policies[0].Limit
	for _, p := range e.policies[1:] {
		if p.Limit > 0 && p.Limit < minLimit {
			minLimit = p.Limit
		}
	}

	remaining := minLimit - currentCost
	if remaining < 0 {
		return 0
	}
	return remaining
}

// currentSpendLocked returns the current spend. Caller must hold e.mu (at least RLock).
func (e *BudgetEnforcer) currentSpendLocked() float64 {
	if e.useInternalTrack {
		return e.internalCost
	}
	return e.tracker.TotalCost()
}

// fireAlerts checks if any thresholds for the given policy should fire and
// calls the alert function. This is safe to call concurrently.
func (e *BudgetEnforcer) fireAlerts(p BudgetPolicy, currentCost float64) {
	if e.alertFunc == nil || p.Limit <= 0 || len(p.Thresholds) == 0 {
		return
	}

	ratio := currentCost / p.Limit

	// Sort thresholds so we fire in ascending order.
	sorted := make([]float64, len(p.Thresholds))
	copy(sorted, p.Thresholds)
	sort.Float64s(sorted)

	for _, threshold := range sorted {
		if ratio >= threshold {
			e.mu.Lock()
			fired, exists := e.firedThresholds[p.Name]
			if !exists {
				fired = make(map[float64]bool)
				e.firedThresholds[p.Name] = fired
			}
			if fired[threshold] {
				e.mu.Unlock()
				continue
			}
			fired[threshold] = true
			e.mu.Unlock()

			e.alertFunc(BudgetAlert{
				PolicyName:  p.Name,
				Threshold:   threshold,
				CurrentCost: currentCost,
				BudgetLimit: p.Limit,
				Timestamp:   time.Now(),
			})
		}
	}
}

// EnforcementMiddleware creates a registry.Middleware that checks the budget
// enforcer before every tool execution.
//   - If Check returns ActionBlock, the tool call is rejected with an error result.
//   - If Check returns ActionDowngrade, the tool call proceeds but the result
//     includes a metadata hint (callers can inspect the action).
//   - If Check returns ActionWarn, the tool call proceeds normally.
//
// After execution, the middleware estimates cost using DefaultEstimate and records
// it to the enforcer.
func EnforcementMiddleware(enforcer *BudgetEnforcer) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Pre-check: evaluate budget before executing.
			action, err := enforcer.Check(ctx)
			if err != nil {
				// Budget exceeded with block action.
				return registry.MakeErrorResult(err.Error()), nil
			}

			if action == ActionBlock {
				// This shouldn't happen (block always returns an error),
				// but handle defensively.
				return registry.MakeErrorResult("finops: budget exceeded, operation blocked"), nil
			}

			// Execute the tool.
			result, callErr := next(ctx, request)

			// Post-execution: estimate and record cost.
			estimate := DefaultEstimate
			inputTokens := EstimateFromRequest(request, estimate)
			outputTokens := 0
			if result != nil {
				outputTokens = EstimateFromResult(result, estimate)
			}

			// Use a rough cost estimate: ~$0.003 per 1K tokens as a default.
			totalTokens := float64(inputTokens + outputTokens)
			cost := totalTokens / 1000.0 * 0.003
			enforcer.RecordCost(cost)

			return result, callErr
		}
	}
}
