package finops_test

import (
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
)

func ExampleNewTracker() {
	tracker := finops.NewTracker()

	tracker.Record(finops.UsageEntry{
		ToolName:     "search",
		Category:     "retrieval",
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Now(),
	})

	fmt.Println(tracker.Total())

	summary := tracker.Summary()
	fmt.Println(summary.TotalInvocations)
	fmt.Println(summary.TotalInputTokens)
	// Output:
	// 150
	// 1
	// 100
}

func ExampleNewScopedTracker() {
	global := finops.NewTracker()
	scoped := finops.NewScopedTracker(global)

	scope := finops.BudgetScope{TenantID: "acme", UserID: "alice"}
	scoped.SetBudget(scope, finops.ScopedBudget{MaxTokens: 1000})

	usage := scoped.Usage(scope)
	fmt.Println(usage.TotalInvocations)
	// Output:
	// 0
}

func ExampleNewCostPolicy() {
	cp := finops.NewCostPolicy(
		finops.WithDollarBudget(10.0),
		finops.WithModelPricing(finops.ModelPricing{
			Model:             "claude-3",
			InputPer1KTokens:  0.003,
			OutputPer1KTokens: 0.015,
		}),
	)

	cost := cp.EstimateCost("claude-3", 1000, 500)
	fmt.Printf("%.4f\n", cost)
	fmt.Println(cp.TotalCost())
	// Output:
	// 0.0105
	// 0
}
