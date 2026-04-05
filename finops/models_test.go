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
		{"exact gpt-4o", "gpt-4o", "gpt-4o"},
		{"exact gemini-2.5-flash", "gemini-2.5-flash", "gemini-2.5-flash"},

		// Alias match
		{"alias opus", "opus", "claude-opus-4-6"},
		{"alias sonnet", "sonnet", "claude-sonnet-4-6"},
		{"alias haiku", "haiku", "claude-haiku-4-5"},
		{"alias gpt4o", "gpt4o", "gpt-4o"},
		{"alias o1-preview", "o1-preview", "o1"},
		{"alias gemini-flash", "gemini-flash", "gemini-2.5-flash"},
		{"alias dated claude", "claude-opus-4-6-20260401", "claude-opus-4-6"},

		// Prefix match
		{"prefix claude-opus-4-6-something", "claude-opus-4-6-custom", "claude-opus-4-6"},

		// Case insensitivity
		{"uppercase", "GPT-4O", "gpt-4o"},
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
			want:         15.0 + 7.5, // 15 * 1 + 75 * 0.1
		},
		{
			name:         "gpt-4o via alias",
			model:        "gpt4o",
			inputTokens:  500_000,
			outputTokens: 500_000,
			want:         2.50*0.5 + 10.0*0.5, // 1.25 + 5.0
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
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"gpt-4o",
		"gemini-2.5-flash",
		"gemini-2.5-pro",
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
