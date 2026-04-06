// Package ab provides a framework for A/B testing prompt variants.
//
// An [ABTest] defines two prompt template variants (A and B) and a [Scorer]
// function to evaluate their outputs against the same input. [Run] executes
// both variants and returns an [ABResult] with per-variant scores and a winner
// determination. [RunN] repeats the test N times and computes aggregate
// statistics with a basic confidence metric.
//
// This package provides the evaluation framework only -- it does not make LLM
// calls. The caller supplies a [Runner] function that takes a prompt string
// and returns the output, enabling use with any backend or mock.
//
// Example:
//
//	test := ab.ABTest{
//	    Name:     "greeting-style",
//	    VariantA: ab.Variant{Name: "formal", Template: "Dear {{.Name}}, ..."},
//	    VariantB: ab.Variant{Name: "casual", Template: "Hey {{.Name}}! ..."},
//	    Scorer:   func(output string) float64 { return float64(len(output)) / 100.0 },
//	    Runner:   func(ctx context.Context, prompt string) (string, error) { return prompt, nil },
//	}
//	result, err := ab.Run(ctx, test, map[string]any{"Name": "World"})
package ab
