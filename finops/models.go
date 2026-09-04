package finops

import "strings"

// Pricing holds per-million-token costs for a model.
type Pricing struct {
	InputPerMToken  float64
	OutputPerMToken float64
}

// DefaultPricing maps canonical model names to their pricing.
// Prices are USD per million tokens. Claude figures verified against
// Anthropic's published model pricing table, cached 2026-06-24. OpenAI and
// Google figures are verified from primary sources per-entry below
// (verified 2026-07-06); anything not individually cited remains from the
// prior 2026-04 baseline and should be treated as potentially stale.
var DefaultPricing = map[string]Pricing{
	// Claude models
	// Claude 5.1 / Opus 5 rows verified against the bundled claude-api model
	// table (cached 2026-06-24) on 2026-09-03.
	"claude-fable-5-1":  {10.0, 50.0},
	"claude-mythos-5-1": {10.0, 50.0},
	"claude-fable-5":    {10.0, 50.0},
	"claude-mythos-5":   {10.0, 50.0},
	"claude-opus-5":     {5.0, 25.0},
	"claude-opus-4-8":   {5.0, 25.0},
	"claude-opus-4-7":   {5.0, 25.0},
	"claude-opus-4-6":   {5.0, 25.0},
	// claude-sonnet-5 has introductory pricing of $2.00/$10.00 per 1M tokens
	// through 2026-08-31; this table uses the standard post-intro pricing.
	"claude-sonnet-5":   {3.0, 15.0},
	"claude-sonnet-4-6": {3.0, 15.0},
	"claude-haiku-4-5":  {1.0, 5.0},
	"claude-opus-4-5":   {15.0, 75.0},
	"claude-sonnet-4-5": {3.0, 15.0},

	// OpenAI / Codex models
	// gpt-5.5 standard pricing verified 2026-07-06:
	// https://developers.openai.com/api/docs/pricing
	"gpt-5.5":           {5.0, 30.0},
	"o1":                {15.0, 60.0},
	"o1-mini":           {3.0, 12.0},
	"o3":                {10.0, 40.0},
	"o3-mini":           {1.10, 4.40},
	"o4-mini":           {1.10, 4.40},
	"codex-mini-latest": {1.50, 6.0},

	// Google Gemini models
	// gemini-3.1-pro standard pricing (<=200k token context tier) verified
	// 2026-07-06: https://ai.google.dev/gemini-api/docs/pricing
	"gemini-3.1-pro": {2.0, 12.0},
}

// modelAliases maps common variations to canonical names.
var modelAliases = map[string]string{
	// Claude aliases
	"claude-opus-4-6-20260401":   "claude-opus-4-6",
	"claude-sonnet-4-6-20260401": "claude-sonnet-4-6",
	"claude-opus-4-5-20250514":   "claude-opus-4-5",
	"claude-sonnet-4-5-20250514": "claude-sonnet-4-5",
	"opus":                       "claude-opus-5",
	"sonnet":                     "claude-sonnet-5",
	"haiku":                      "claude-haiku-4-5",
	"fable":                      "claude-fable-5-1",

	// OpenAI aliases
	"o1-preview": "o1",

	// Gemini aliases
	"gemini-pro": "gemini-3.1-pro",
}

// NormalizeModelName maps a raw model identifier to its canonical name.
func NormalizeModelName(raw string) string {
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	// Claude Code spells the 1M-context opt-in as a "[1m]" suffix on the
	// model ID; pricing is identical, so strip it before lookup.
	lower = strings.TrimSuffix(lower, "[1m]")

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
