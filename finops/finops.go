// Package finops provides token accounting, budget policies, and usage tracking
// for MCP tool invocations.
package finops

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// UsageEntry records token usage for a single tool invocation.
type UsageEntry struct {
	ToolName     string        `json:"tool_name"`
	Category     string        `json:"category"`
	Model        string        `json:"model,omitempty"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Duration     time.Duration `json:"duration_ns"`
	Timestamp    time.Time     `json:"timestamp"`
}

// UsageSummary aggregates token usage across invocations.
type UsageSummary struct {
	TotalInputTokens  int64            `json:"total_input_tokens"`
	TotalOutputTokens int64            `json:"total_output_tokens"`
	TotalInvocations  int64            `json:"total_invocations"`
	ByTool            map[string]int64 `json:"by_tool"`
	ByCategory        map[string]int64 `json:"by_category"`
}

// BudgetExceededError is returned when a tool invocation would exceed the token budget.
type BudgetExceededError struct {
	Limit int64
	Used  int64
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("finops: token budget exceeded (limit=%d, used=%d)", e.Limit, e.Used)
}

// Config configures the finops middleware.
type Config struct {
	// TokenBudget is the max total tokens (input+output) before rejecting. 0 = unlimited.
	TokenBudget int64
	// EstimateFunc overrides the default token estimation. If nil, uses DefaultEstimate.
	EstimateFunc func(text string) int
	// OnBudgetExceeded is called when budget is hit. If nil, returns error result.
	OnBudgetExceeded func(entry UsageEntry, summary UsageSummary)
	// CostPolicy optionally enforces a dollar-cost budget. If nil, no cost budget is checked.
	CostPolicy *CostPolicy
}

// Middleware returns a registry.Middleware that records token usage for every tool invocation.
// The middleware:
// 1. Pre-call: checks budget against current total
// 2. Estimates input tokens from request arguments
// 3. Calls the next handler
// 4. Post-call: estimates output tokens from result content
// 5. Records a UsageEntry to the tracker
// If the budget is exceeded before execution, returns an error result.
func Middleware(tracker *Tracker) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		category := td.Category
		if category == "" {
			category = "unknown"
		}
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			estimate := tracker.config.EstimateFunc
			if estimate == nil {
				estimate = DefaultEstimate
			}

			// Check budget before execution.
			if tracker.config.TokenBudget > 0 && tracker.Total() >= tracker.config.TokenBudget {
				summary := tracker.Summary()
				entry := UsageEntry{ToolName: name, Category: category, Timestamp: time.Now()}
				if tracker.config.OnBudgetExceeded != nil {
					tracker.config.OnBudgetExceeded(entry, summary)
				}
				return registry.MakeErrorResult((&BudgetExceededError{
					Limit: tracker.config.TokenBudget,
					Used:  summary.TotalInputTokens + summary.TotalOutputTokens,
				}).Error()), nil
			}

			// Estimate input tokens.
			inputTokens := EstimateFromRequest(request, estimate)

			start := time.Now()
			result, err := next(ctx, request)
			duration := time.Since(start)

			// Estimate output tokens.
			outputTokens := 0
			if result != nil {
				outputTokens = EstimateFromResult(result, estimate)
			}

			// Record usage.
			entry := UsageEntry{
				ToolName:     name,
				Category:     category,
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				Duration:     duration,
				Timestamp:    time.Now(),
			}
			tracker.Record(entry)

			// Record dollar cost if CostPolicy is configured.
			if cp := tracker.config.CostPolicy; cp != nil {
				cost := cp.EstimateCost("", inputTokens, outputTokens)
				if err2 := cp.RecordCost(cost); err2 != nil {
					return registry.MakeErrorResult(err2.Error()), nil
				}
			}

			return result, err
		}
	}
}
