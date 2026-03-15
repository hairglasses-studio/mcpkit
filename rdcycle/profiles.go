package rdcycle

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hairglasses-studio/mcpkit/finops"
)

// BudgetProfile defines per-cycle and daily spending limits that compose
// existing finops primitives into a reusable preset.
type BudgetProfile struct {
	Name            string                `json:"name"`
	MaxIterations   int                   `json:"max_iterations"`
	DollarBudget    float64               `json:"dollar_budget"`
	DailyDollarCap  float64               `json:"daily_dollar_cap"`
	TokenBudget     int                   `json:"token_budget"`
	MaxTokensPerReq int                   `json:"max_tokens_per_req"`
	ModelPricing    []finops.ModelPricing  `json:"model_pricing"`
}

// PersonalProfile returns a conservative budget for Claude Max subscription use.
// $5/cycle, $20/day, 50 iterations max, 4096 tokens per request.
func PersonalProfile() BudgetProfile {
	return BudgetProfile{
		Name:            "personal",
		MaxIterations:   50,
		DollarBudget:    5.0,
		DailyDollarCap:  20.0,
		TokenBudget:     500000,
		MaxTokensPerReq: 4096,
		ModelPricing: []finops.ModelPricing{
			{Model: "claude-opus-4-6", InputPer1KTokens: 0.015, OutputPer1KTokens: 0.075},
			{Model: "claude-sonnet-4-6", InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
			{Model: "claude-haiku-4-5", InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.004},
		},
	}
}

// WorkAPIProfile returns a higher-limit budget for direct API credit use.
// $50/cycle, $200/day, 200 iterations max, 8192 tokens per request.
func WorkAPIProfile() BudgetProfile {
	return BudgetProfile{
		Name:            "work-api",
		MaxIterations:   200,
		DollarBudget:    50.0,
		DailyDollarCap:  200.0,
		TokenBudget:     5000000,
		MaxTokensPerReq: 8192,
		ModelPricing: []finops.ModelPricing{
			{Model: "claude-opus-4-6", InputPer1KTokens: 0.015, OutputPer1KTokens: 0.075},
			{Model: "claude-sonnet-4-6", InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
			{Model: "claude-haiku-4-5", InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.004},
		},
	}
}

// LoadProfile reads a BudgetProfile from a JSON file.
func LoadProfile(path string) (BudgetProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BudgetProfile{}, fmt.Errorf("rdcycle: load profile: %w", err)
	}
	var p BudgetProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return BudgetProfile{}, fmt.Errorf("rdcycle: parse profile: %w", err)
	}
	return p, nil
}

// SaveProfile writes a BudgetProfile to a JSON file.
func SaveProfile(path string, p BudgetProfile) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("rdcycle: marshal profile: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("rdcycle: write profile: %w", err)
	}
	return nil
}

// BuildFinOpsStack constructs a Tracker, CostPolicy, and WindowedTracker from
// a BudgetProfile. The Tracker enforces per-cycle token budget, CostPolicy
// enforces per-cycle dollar budget, and WindowedTracker handles daily caps.
func BuildFinOpsStack(p BudgetProfile) (*finops.Tracker, *finops.CostPolicy, *finops.WindowedTracker) {
	// Build cost policy with dollar budget and model pricing.
	cpOpts := []finops.CostPolicyOption{
		finops.WithDollarBudget(p.DollarBudget),
	}
	for _, mp := range p.ModelPricing {
		cpOpts = append(cpOpts, finops.WithModelPricing(mp))
	}
	cp := finops.NewCostPolicy(cpOpts...)

	// Build tracker with token budget and cost policy.
	cfg := finops.Config{
		TokenBudget: int64(p.TokenBudget),
		CostPolicy:  cp,
	}
	tracker := finops.NewTracker(cfg)

	// Build daily windowed tracker with same config.
	dailyCfg := finops.Config{
		TokenBudget: 0, // daily budget enforced via CostPolicy
		CostPolicy:  finops.NewCostPolicy(finops.WithDollarBudget(p.DailyDollarCap)),
	}
	wt := finops.NewWindowedTracker(dailyCfg, finops.ResetDaily)

	return tracker, cp, wt
}
