package finops

import (
	"math"
	"strings"
	"testing"
)

func TestCostPolicy_EstimateCost(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy(
		WithModelPricing(ModelPricing{
			Model:             "gpt-4",
			InputPer1KTokens:  0.03,
			OutputPer1KTokens: 0.06,
		}),
	)

	// 1000 input tokens at $0.03/1K = $0.03
	// 500 output tokens at $0.06/1K = $0.03
	// total = $0.06
	cost := cp.EstimateCost("gpt-4", 1000, 500)
	if cost < 0.059 || cost > 0.061 {
		t.Errorf("expected cost ~0.06, got %f", cost)
	}
}

func TestCostPolicy_EstimateCost_UnknownModel(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy()
	cost := cp.EstimateCost("unknown-model", 1000, 1000)
	if cost != 0 {
		t.Errorf("expected 0 for unknown model, got %f", cost)
	}
}

func TestCostPolicy_EstimateCost_EmptyModel(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy(
		WithModelPricing(ModelPricing{
			Model:            "gpt-4",
			InputPer1KTokens: 0.03,
		}),
	)
	cost := cp.EstimateCost("", 1000, 0)
	if cost != 0 {
		t.Errorf("expected 0 for empty model, got %f", cost)
	}
}

func TestCostPolicy_DollarBudgetEnforcement(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy(
		WithDollarBudget(0.05),
		WithModelPricing(ModelPricing{
			Model:             "claude-3",
			InputPer1KTokens:  0.008,
			OutputPer1KTokens: 0.024,
		}),
	)

	// Record small cost — should succeed.
	if err := cp.RecordCost(0.02); err != nil {
		t.Errorf("expected no error for cost within budget, got: %v", err)
	}

	// Record cost that pushes over budget.
	if err := cp.RecordCost(0.04); err == nil {
		t.Error("expected error when exceeding dollar budget, got nil")
	} else if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected 'budget exceeded' in error, got: %q", err.Error())
	}
}

func TestCostPolicy_TotalCost(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy()
	if cp.TotalCost() != 0 {
		t.Errorf("expected TotalCost=0 on new policy, got %f", cp.TotalCost())
	}

	cp.RecordCost(0.10) //nolint:errcheck
	cp.RecordCost(0.05) //nolint:errcheck

	total := cp.TotalCost()
	if total < 0.149 || total > 0.151 {
		t.Errorf("expected TotalCost~0.15, got %f", total)
	}
}

func TestCostPolicy_RemainingBudget_NoBudget(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy()
	if cp.RemainingBudget() != math.MaxFloat64 {
		t.Errorf("expected MaxFloat64 when no budget set, got %f", cp.RemainingBudget())
	}
}

func TestCostPolicy_RemainingBudget_WithBudget(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy(WithDollarBudget(1.00))
	cp.RecordCost(0.30) //nolint:errcheck

	remaining := cp.RemainingBudget()
	if remaining < 0.699 || remaining > 0.701 {
		t.Errorf("expected remaining~0.70, got %f", remaining)
	}
}

func TestCostPolicy_RemainingBudget_Exhausted(t *testing.T) {
	t.Parallel()

	cp := NewCostPolicy(WithDollarBudget(0.10))
	cp.RecordCost(0.20) //nolint:errcheck // intentionally over budget

	if cp.RemainingBudget() != 0 {
		t.Errorf("expected RemainingBudget=0 when over budget, got %f", cp.RemainingBudget())
	}
}

func TestDollarBudgetExceededError_Message(t *testing.T) {
	t.Parallel()

	err := &DollarBudgetExceededError{Limit: 1.00, Used: 1.25}
	msg := err.Error()
	if !strings.Contains(msg, "budget exceeded") {
		t.Errorf("expected 'budget exceeded' in message, got: %q", msg)
	}
}
