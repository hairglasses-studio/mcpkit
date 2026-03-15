package finops

import (
	"fmt"
	"math"
	"sync"
)

// ModelPricing holds per-model pricing information.
type ModelPricing struct {
	Model             string
	InputPer1KTokens  float64
	OutputPer1KTokens float64
}

// DollarBudgetExceededError is returned when a tool invocation would exceed the dollar budget.
type DollarBudgetExceededError struct {
	Limit float64
	Used  float64
}

func (e *DollarBudgetExceededError) Error() string {
	return fmt.Sprintf("finops: dollar budget exceeded (limit=$%.4f, used=$%.4f)", e.Limit, e.Used)
}

// CostPolicy tracks dollar costs and optionally enforces a dollar budget. Thread-safe.
type CostPolicy struct {
	mu           sync.RWMutex
	pricing      map[string]ModelPricing
	dollarBudget float64
	totalCost    float64
}

// CostPolicyOption configures a CostPolicy.
type CostPolicyOption func(*CostPolicy)

// WithDollarBudget sets a maximum dollar budget. 0 means no limit.
func WithDollarBudget(dollars float64) CostPolicyOption {
	return func(cp *CostPolicy) {
		cp.dollarBudget = dollars
	}
}

// WithModelPricing adds or replaces pricing for a model.
func WithModelPricing(p ModelPricing) CostPolicyOption {
	return func(cp *CostPolicy) {
		cp.pricing[p.Model] = p
	}
}

// NewCostPolicy creates a new CostPolicy with optional configuration.
func NewCostPolicy(opts ...CostPolicyOption) *CostPolicy {
	cp := &CostPolicy{
		pricing: make(map[string]ModelPricing),
	}
	for _, opt := range opts {
		opt(cp)
	}
	return cp
}

// EstimateCost returns the estimated dollar cost for the given token counts and model.
// If the model is unknown or empty, returns 0.
func (cp *CostPolicy) EstimateCost(model string, inputTokens, outputTokens int) float64 {
	cp.mu.RLock()
	p, ok := cp.pricing[model]
	cp.mu.RUnlock()
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) / 1000.0 * p.InputPer1KTokens
	outputCost := float64(outputTokens) / 1000.0 * p.OutputPer1KTokens
	return inputCost + outputCost
}

// RecordCost adds cost to the running total. Returns an error if the dollar budget
// would be exceeded after recording.
func (cp *CostPolicy) RecordCost(cost float64) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.totalCost += cost
	if cp.dollarBudget > 0 && cp.totalCost > cp.dollarBudget {
		return &DollarBudgetExceededError{
			Limit: cp.dollarBudget,
			Used:  cp.totalCost,
		}
	}
	return nil
}

// TotalCost returns the accumulated dollar cost so far.
func (cp *CostPolicy) TotalCost() float64 {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.totalCost
}

// RemainingBudget returns the remaining dollar budget.
// Returns math.MaxFloat64 if no budget is set.
func (cp *CostPolicy) RemainingBudget() float64 {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	if cp.dollarBudget <= 0 {
		return math.MaxFloat64
	}
	remaining := cp.dollarBudget - cp.totalCost
	if remaining < 0 {
		return 0
	}
	return remaining
}
