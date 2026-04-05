package finops

import "strings"

// Pricing holds per-million-token costs for a model.
type Pricing struct {
	InputPerMToken  float64
	OutputPerMToken float64
}

// DefaultPricing maps canonical model names to their pricing.
// Prices are USD per million tokens, current as of 2026-04.
var DefaultPricing = map[string]Pricing{
	// Claude models
	"claude-opus-4-6":   {15.0, 75.0},
	"claude-sonnet-4-6": {3.0, 15.0},
	"claude-haiku-4-5":  {0.80, 4.0},
	"claude-opus-4-5":   {15.0, 75.0},
	"claude-sonnet-4-5": {3.0, 15.0},

	// OpenAI / Codex models
	"gpt-4o":            {2.50, 10.0},
	"gpt-4o-mini":       {0.15, 0.60},
	"gpt-4-turbo":       {10.0, 30.0},
	"o1":                {15.0, 60.0},
	"o1-mini":           {3.0, 12.0},
	"o3":                {10.0, 40.0},
	"o3-mini":           {1.10, 4.40},
	"o4-mini":           {1.10, 4.40},
	"codex-mini-latest": {1.50, 6.0},

	// Google Gemini models
	"gemini-2.5-pro":        {1.25, 10.0},
	"gemini-2.5-flash":      {0.30, 2.50},
	"gemini-2.0-flash":      {0.10, 0.40},
	"gemini-2.0-flash-lite": {0.075, 0.30},
	"gemini-1.5-pro":        {1.25, 5.0},
	"gemini-1.5-flash":      {0.075, 0.30},
}

// modelAliases maps common variations to canonical names.
var modelAliases = map[string]string{
	// Claude aliases
	"claude-opus-4-6-20260401":   "claude-opus-4-6",
	"claude-sonnet-4-6-20260401": "claude-sonnet-4-6",
	"claude-haiku-4-5-20251001":  "claude-haiku-4-5",
	"claude-opus-4-5-20250514":   "claude-opus-4-5",
	"claude-sonnet-4-5-20250514": "claude-sonnet-4-5",
	"opus":                       "claude-opus-4-6",
	"sonnet":                     "claude-sonnet-4-6",
	"haiku":                      "claude-haiku-4-5",

	// OpenAI aliases
	"gpt-4o-2024-11-20": "gpt-4o",
	"gpt4o":             "gpt-4o",
	"o1-preview":        "o1",

	// Gemini aliases
	"gemini-flash":      "gemini-2.5-flash",
	"gemini-pro":        "gemini-2.5-pro",
	"gemini-flash-lite": "gemini-2.0-flash-lite",
}

// NormalizeModelName maps a raw model identifier to its canonical name.
func NormalizeModelName(raw string) string {
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)

	if canonical, ok := modelAliases[lower]; ok {
		return canonical
	}
	if _, ok := DefaultPricing[lower]; ok {
		return lower
	}
	for key := range DefaultPricing {
		if strings.HasPrefix(lower, key) {
			return key
		}
	}
	return raw
}

// ModelCost computes USD cost for a model and token counts using DefaultPricing.
// Returns 0.0 for unknown models.
func ModelCost(model string, inputTokens, outputTokens int64) float64 {
	canonical := NormalizeModelName(model)
	p, ok := DefaultPricing[canonical]
	if !ok {
		return 0.0
	}
	return float64(inputTokens)/1_000_000*p.InputPerMToken +
		float64(outputTokens)/1_000_000*p.OutputPerMToken
}
