package ab

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"text/template"
	"time"
)

// Variant represents one side of an A/B test.
type Variant struct {
	// Name identifies the variant (e.g., "formal", "casual").
	Name string `json:"name"`
	// Template is a Go text/template string that produces the prompt.
	// Template data is passed via the input map.
	Template string `json:"template"`
}

// Render executes the template with the given data and returns the prompt.
func (v Variant) Render(data map[string]any) (string, error) {
	if v.Template == "" {
		return "", fmt.Errorf("ab: variant %q has empty template", v.Name)
	}
	tmpl, err := template.New(v.Name).Parse(v.Template)
	if err != nil {
		return "", fmt.Errorf("ab: variant %q template parse: %w", v.Name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("ab: variant %q template exec: %w", v.Name, err)
	}
	return buf.String(), nil
}

// Scorer evaluates the quality of a prompt output and returns a score
// between 0.0 and 1.0.
type Scorer func(output string) float64

// Runner executes a prompt and returns the output. This abstraction allows
// testing with mocks, LLM backends, or any other evaluator.
type Runner func(ctx context.Context, prompt string) (string, error)

// ABTest defines the configuration for an A/B prompt comparison.
type ABTest struct {
	// Name identifies this test.
	Name string `json:"name"`
	// VariantA is the first prompt variant.
	VariantA Variant `json:"variant_a"`
	// VariantB is the second prompt variant.
	VariantB Variant `json:"variant_b"`
	// Scorer evaluates the quality of each variant's output.
	Scorer Scorer `json:"-"`
	// Runner executes a rendered prompt and returns the output.
	Runner Runner `json:"-"`
}

// Validate checks that the test configuration is complete.
func (t ABTest) Validate() error {
	if t.Name == "" {
		return errors.New("ab: test name is required")
	}
	if t.VariantA.Name == "" || t.VariantA.Template == "" {
		return errors.New("ab: variant A name and template are required")
	}
	if t.VariantB.Name == "" || t.VariantB.Template == "" {
		return errors.New("ab: variant B name and template are required")
	}
	if t.Scorer == nil {
		return errors.New("ab: scorer is required")
	}
	if t.Runner == nil {
		return errors.New("ab: runner is required")
	}
	return nil
}

// VariantResult captures the outcome for a single variant in one run.
type VariantResult struct {
	// Name is the variant name.
	Name string `json:"name"`
	// Prompt is the rendered prompt text.
	Prompt string `json:"prompt"`
	// Output is what the runner returned.
	Output string `json:"output"`
	// Score is the scorer's evaluation of the output.
	Score float64 `json:"score"`
	// Duration is how long the runner took.
	Duration time.Duration `json:"duration"`
	// Error is set if the runner failed.
	Error string `json:"error,omitempty"`
}

// ABResult is the outcome of a single A/B test run.
type ABResult struct {
	// Name is the test name.
	Name string `json:"name"`
	// A is the result for variant A.
	A VariantResult `json:"a"`
	// B is the result for variant B.
	B VariantResult `json:"b"`
	// Winner is the name of the winning variant, or "" for a tie.
	Winner string `json:"winner"`
	// ScoreDelta is ScoreA - ScoreB. Positive means A is better.
	ScoreDelta float64 `json:"score_delta"`
}

// AggregateResult is the outcome of running an A/B test multiple times.
type AggregateResult struct {
	// Name is the test name.
	Name string `json:"name"`
	// Runs is the number of iterations.
	Runs int `json:"runs"`
	// AvgScoreA is the mean score for variant A across all runs.
	AvgScoreA float64 `json:"avg_score_a"`
	// AvgScoreB is the mean score for variant B across all runs.
	AvgScoreB float64 `json:"avg_score_b"`
	// Winner is the variant with the higher average score, or "" for a tie.
	Winner string `json:"winner"`
	// Confidence is a basic metric: abs(avgA - avgB) / max(stddev, epsilon).
	// Higher values indicate more separation between variants.
	// This is NOT a statistical p-value; it is a simple signal/noise ratio.
	Confidence float64 `json:"confidence"`
	// WinsA is the number of runs where A scored higher.
	WinsA int `json:"wins_a"`
	// WinsB is the number of runs where B scored higher.
	WinsB int `json:"wins_b"`
	// Ties is the number of runs where scores were equal.
	Ties int `json:"ties"`
	// Results contains all individual run results.
	Results []ABResult `json:"results"`
}

// Run executes a single A/B test with the given input data. Both variants
// are rendered, run through the Runner, and scored.
func Run(ctx context.Context, test ABTest, input map[string]any) (ABResult, error) {
	if err := test.Validate(); err != nil {
		return ABResult{}, err
	}

	result := ABResult{Name: test.Name}

	// Run variant A
	result.A = runVariant(ctx, test.VariantA, input, test.Runner, test.Scorer)

	// Run variant B
	result.B = runVariant(ctx, test.VariantB, input, test.Runner, test.Scorer)

	// Determine winner
	result.ScoreDelta = result.A.Score - result.B.Score
	switch {
	case result.A.Score > result.B.Score:
		result.Winner = result.A.Name
	case result.B.Score > result.A.Score:
		result.Winner = result.B.Name
	default:
		result.Winner = "" // tie
	}

	return result, nil
}

// RunN executes the A/B test N times and computes aggregate statistics.
func RunN(ctx context.Context, test ABTest, input map[string]any, n int) (AggregateResult, error) {
	if n <= 0 {
		return AggregateResult{}, errors.New("ab: n must be positive")
	}
	if err := test.Validate(); err != nil {
		return AggregateResult{}, err
	}

	agg := AggregateResult{
		Name:    test.Name,
		Runs:    n,
		Results: make([]ABResult, 0, n),
	}

	var sumA, sumB float64
	scoresA := make([]float64, 0, n)
	scoresB := make([]float64, 0, n)

	for i := 0; i < n; i++ {
		r, err := Run(ctx, test, input)
		if err != nil {
			return AggregateResult{}, fmt.Errorf("ab: run %d: %w", i, err)
		}
		agg.Results = append(agg.Results, r)

		sumA += r.A.Score
		sumB += r.B.Score
		scoresA = append(scoresA, r.A.Score)
		scoresB = append(scoresB, r.B.Score)

		switch {
		case r.A.Score > r.B.Score:
			agg.WinsA++
		case r.B.Score > r.A.Score:
			agg.WinsB++
		default:
			agg.Ties++
		}
	}

	agg.AvgScoreA = sumA / float64(n)
	agg.AvgScoreB = sumB / float64(n)

	// Determine overall winner
	switch {
	case agg.AvgScoreA > agg.AvgScoreB:
		agg.Winner = test.VariantA.Name
	case agg.AvgScoreB > agg.AvgScoreA:
		agg.Winner = test.VariantB.Name
	default:
		agg.Winner = ""
	}

	// Basic confidence: signal-to-noise ratio
	// Uses pooled standard deviation of score differences
	diffs := make([]float64, n)
	var sumDiff float64
	for i := 0; i < n; i++ {
		diffs[i] = scoresA[i] - scoresB[i]
		sumDiff += diffs[i]
	}
	meanDiff := sumDiff / float64(n)

	var variance float64
	for _, d := range diffs {
		delta := d - meanDiff
		variance += delta * delta
	}
	if n > 1 {
		variance /= float64(n - 1)
	}
	stddev := math.Sqrt(variance)

	const epsilon = 1e-10
	if stddev < epsilon {
		if math.Abs(meanDiff) < epsilon {
			agg.Confidence = 0
		} else {
			agg.Confidence = 1.0 // perfect consistency, non-zero difference
		}
	} else {
		agg.Confidence = math.Abs(meanDiff) / stddev
	}

	return agg, nil
}

func runVariant(ctx context.Context, v Variant, input map[string]any, runner Runner, scorer Scorer) VariantResult {
	result := VariantResult{Name: v.Name}

	prompt, err := v.Render(input)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Prompt = prompt

	start := time.Now()
	output, err := runner(ctx, prompt)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Output = output
	result.Score = scorer(output)

	return result
}
