package finops

import (
	"math"
	"testing"
)

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		// Exact match
		{"exact claude-opus-4-6", "claude-opus-4-6", "claude-opus-4-6"},
		{"exact gpt-5.5", "gpt-5.5", "gpt-5.5"},
		{"exact gemini-3.1-pro", "gemini-3.1-pro", "gemini-3.1-pro"},

		// Alias match
		{"alias opus", "opus", "claude-opus-5"},
		{"alias sonnet", "sonnet", "claude-sonnet-5"},
		{"alias haiku", "haiku", "claude-haiku-4-5"},
		{"alias fable", "fable", "claude-fable-5-1"},
		{"1m suffix stripped", "claude-fable-5-1[1m]", "claude-fable-5-1"},
		{"1m suffix on alias", "opus[1m]", "claude-opus-5"},
		{"alias o1-preview", "o1-preview", "o1"},
		{"alias gemini-pro", "gemini-pro", "gemini-3.1-pro"},
		{"alias dated claude", "claude-opus-4-6-20260401", "claude-opus-4-6"},

		// Prefix match
		{"prefix claude-opus-4-6-something", "claude-opus-4-6-custom", "claude-opus-4-6"},

		// Case insensitivity
		{"uppercase", "GPT-5.5", "gpt-5.5"},
		{"mixed case", "Claude-Opus-4-6", "claude-opus-4-6"},

		// Empty
		{"empty string", "", ""},

		// Unknown
		{"unknown model", "llama-3-70b", "llama-3-70b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeModelName(tt.raw)
			if got != tt.want {
				t.Errorf("NormalizeModelName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestModelCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int64
		outputTokens int64
		want         float64
	}{
		{
			name:         "claude-opus-4-6 cost",
			model:        "claude-opus-4-6",
			inputTokens:  1_000_000,
			outputTokens: 100_000,
			want:         5.0 + 2.5, // 5 * 1 + 25 * 0.1
		},
		{
			name:         "gpt-5.5",
			model:        "gpt-5.5",
			inputTokens:  500_000,
			outputTokens: 500_000,
			want:         5.0*0.5 + 30.0*0.5, // 2.5 + 15.0
		},
		{
			name:         "unknown model returns 0",
			model:        "nonexistent-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         0.0,
		},
		{
			name:         "zero tokens",
			model:        "claude-opus-4-6",
			inputTokens:  0,
			outputTokens: 0,
			want:         0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ModelCost(tt.model, tt.inputTokens, tt.outputTokens)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ModelCost(%q, %d, %d) = %f, want %f",
					tt.model, tt.inputTokens, tt.outputTokens, got, tt.want)
			}
		})
	}
}

func TestDefaultPricingCompleteness(t *testing.T) {
	required := []string{
		"claude-fable-5-1",
		"claude-mythos-5-1",
		"claude-fable-5",
		"claude-mythos-5",
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-sonnet-5",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"gpt-5.5",
		"gemini-3.1-pro",
		"o3-mini",
		"codex-mini-latest",
	}

	for _, model := range required {
		t.Run(model, func(t *testing.T) {
			p, ok := DefaultPricing[model]
			if !ok {
				t.Fatalf("DefaultPricing missing required model %q", model)
			}
			if p.InputPerMToken <= 0 {
				t.Errorf("InputPerMToken for %q should be positive, got %f", model, p.InputPerMToken)
			}
			if p.OutputPerMToken <= 0 {
				t.Errorf("OutputPerMToken for %q should be positive, got %f", model, p.OutputPerMToken)
			}
			if p.OutputPerMToken < p.InputPerMToken {
				t.Errorf("OutputPerMToken (%f) < InputPerMToken (%f) for %q — unusual",
					p.OutputPerMToken, p.InputPerMToken, model)
			}
		})
	}
}
