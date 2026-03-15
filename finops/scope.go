//go:build !official_sdk

package finops

import (
	"context"
	"fmt"
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TenantResolver resolves tenant/user/session identity from a context.
// This avoids finops importing the security package (Layer 2 cannot import Layer 3).
// Returns ok=false if no identity is available; in that case the call is not scoped.
type TenantResolver func(ctx context.Context) (tenantID, userID, sessionID string, ok bool)

// BudgetScope identifies a specific tenant/user/session combination.
type BudgetScope struct {
	TenantID  string
	UserID    string
	SessionID string
}

// ScopedBudget defines token and dollar limits for a specific scope.
type ScopedBudget struct {
	MaxTokens  int
	MaxDollars float64
}

// ScopedTracker maintains per-scope Tracker instances alongside a global tracker.
// Thread-safe.
type ScopedTracker struct {
	mu      sync.RWMutex
	global  *Tracker
	scoped  map[BudgetScope]*Tracker
	budgets map[BudgetScope]ScopedBudget
}

// NewScopedTracker creates a ScopedTracker that records to global and per-scope trackers.
func NewScopedTracker(global *Tracker) *ScopedTracker {
	return &ScopedTracker{
		global:  global,
		scoped:  make(map[BudgetScope]*Tracker),
		budgets: make(map[BudgetScope]ScopedBudget),
	}
}

// SetBudget configures a budget for a specific scope. Replaces any existing budget.
func (st *ScopedTracker) SetBudget(scope BudgetScope, budget ScopedBudget) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.budgets[scope] = budget
}

// GetTracker returns the Tracker for the given scope, creating one if it does not exist.
func (st *ScopedTracker) GetTracker(scope BudgetScope) *Tracker {
	st.mu.RLock()
	t, ok := st.scoped[scope]
	st.mu.RUnlock()
	if ok {
		return t
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	// Double-checked locking.
	if t, ok = st.scoped[scope]; ok {
		return t
	}
	t = NewTracker()
	st.scoped[scope] = t
	return t
}

// Usage returns the UsageSummary for a specific scope.
func (st *ScopedTracker) Usage(scope BudgetScope) UsageSummary {
	return st.GetTracker(scope).Summary()
}

// ScopedBudgetExceededError is returned when a scoped budget is exceeded.
type ScopedBudgetExceededError struct {
	Scope   BudgetScope
	Kind    string // "token" or "dollar"
	Limit   int64
	Used    int64
	LimitF  float64
	UsedF   float64
}

func (e *ScopedBudgetExceededError) Error() string {
	if e.Kind == "dollar" {
		return fmt.Sprintf("finops: scoped dollar budget exceeded for tenant=%q user=%q session=%q (limit=$%.4f, used=$%.4f)",
			e.Scope.TenantID, e.Scope.UserID, e.Scope.SessionID, e.LimitF, e.UsedF)
	}
	return fmt.Sprintf("finops: scoped token budget exceeded for tenant=%q user=%q session=%q (limit=%d, used=%d)",
		e.Scope.TenantID, e.Scope.UserID, e.Scope.SessionID, e.Limit, e.Used)
}

// ScopedMiddleware returns a registry.Middleware that:
//  1. Resolves tenant context using the provided TenantResolver.
//  2. Records usage to both the global tracker and the per-scope tracker.
//  3. Checks scoped token and dollar budgets after recording.
//
// If resolve is nil or returns ok=false, the call passes through without scoping.
func ScopedMiddleware(st *ScopedTracker, resolve TenantResolver) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		category := td.Category
		if category == "" {
			category = "unknown"
		}
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Resolve scope.
			var scope BudgetScope
			var scoped bool
			if resolve != nil {
				tenantID, userID, sessionID, ok := resolve(ctx)
				if ok {
					scope = BudgetScope{TenantID: tenantID, UserID: userID, SessionID: sessionID}
					scoped = true
				}
			}

			// Check scoped token budget before execution.
			if scoped {
				st.mu.RLock()
				budget, hasBudget := st.budgets[scope]
				st.mu.RUnlock()
				if hasBudget && budget.MaxTokens > 0 {
					t := st.GetTracker(scope)
					if t.Total() >= int64(budget.MaxTokens) {
						summary := t.Summary()
						used := summary.TotalInputTokens + summary.TotalOutputTokens
						return registry.MakeErrorResult((&ScopedBudgetExceededError{
							Scope: scope,
							Kind:  "token",
							Limit: int64(budget.MaxTokens),
							Used:  used,
						}).Error()), nil
					}
				}
			}

			estimate := st.global.config.EstimateFunc
			if estimate == nil {
				estimate = DefaultEstimate
			}

			inputTokens := EstimateFromRequest(req, estimate)

			result, err := next(ctx, req)

			outputTokens := 0
			if result != nil {
				outputTokens = EstimateFromResult(result, estimate)
			}

			entry := UsageEntry{
				ToolName:     name,
				Category:     category,
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
			}

			// Record to global tracker.
			st.global.Record(entry)

			// Record to scoped tracker and check scoped budget.
			if scoped {
				scopedT := st.GetTracker(scope)
				scopedT.Record(entry)

				st.mu.RLock()
				budget, hasBudget := st.budgets[scope]
				st.mu.RUnlock()

				if hasBudget {
					if budget.MaxTokens > 0 && scopedT.Total() > int64(budget.MaxTokens) {
						summary := scopedT.Summary()
						used := summary.TotalInputTokens + summary.TotalOutputTokens
						return registry.MakeErrorResult((&ScopedBudgetExceededError{
							Scope: scope,
							Kind:  "token",
							Limit: int64(budget.MaxTokens),
							Used:  used,
						}).Error()), nil
					}
				}
			}

			return result, err
		}
	}
}
