//go:build !official_sdk

package eval

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Run executes all cases in the suite against the provided registry and
// returns an aggregate Summary.
func Run(ctx context.Context, suite Suite, reg *registry.ToolRegistry) Summary {
	threshold := suite.Threshold
	if threshold == 0 {
		threshold = 1.0
	}

	start := time.Now()
	results := make([]Result, 0, len(suite.Cases))
	var totalScore float64
	passed := 0

	for _, c := range suite.Cases {
		r := runCase(ctx, c, suite.Scorers, suite.ResultScorers, threshold, reg)
		results = append(results, r)
		totalScore += r.AverageScore()
		if r.Passed {
			passed++
		}
	}

	avgScore := 0.0
	if len(results) > 0 {
		avgScore = totalScore / float64(len(results))
	}

	return Summary{
		Suite:    suite.Name,
		Total:    len(results),
		Passed:   passed,
		Failed:   len(results) - passed,
		AvgScore: avgScore,
		Duration: time.Since(start),
		Results:  results,
	}
}

// RunT is a test-friendly wrapper around Run. It logs per-case scores and
// calls t.Fatalf if any case fails.
func RunT(t *testing.T, ctx context.Context, suite Suite, reg *registry.ToolRegistry) Summary {
	t.Helper()

	summary := Run(ctx, suite, reg)

	for _, r := range summary.Results {
		if r.Passed {
			t.Logf("PASS %s (avg=%.2f)", r.Case.Name, r.AverageScore())
		} else {
			t.Errorf("FAIL %s (avg=%.2f)", r.Case.Name, r.AverageScore())
			for _, s := range r.Scores {
				if s.Value < 1.0 {
					t.Errorf("  scorer %s: %.2f — %s", s.Scorer, s.Value, s.Reason)
				}
			}
		}
	}

	if summary.Failed > 0 {
		t.Fatalf("eval suite %q: %d/%d cases failed", suite.Name, summary.Failed, summary.Total)
	}

	return summary
}

func runCase(ctx context.Context, c Case, scorers []Scorer, resultScorers []ResultScorer, threshold float64, reg *registry.ToolRegistry) Result {
	start := time.Now()

	td, ok := reg.GetTool(c.Tool)
	if !ok {
		return Result{
			Case:     c,
			Scores:   []Score{{Scorer: "system", Value: 0.0, Reason: fmt.Sprintf("tool %q not found", c.Tool)}},
			Error:    true,
			Duration: time.Since(start),
			Passed:   false,
		}
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = c.Tool
	if c.Args != nil {
		req.Params.Arguments = c.Args
	}

	result, err := td.Handler(ctx, req)

	output := ""
	isErr := false
	if err != nil {
		output = err.Error()
		isErr = true
	} else if result != nil {
		output = extractText(result)
		isErr = registry.IsResultError(result)
	}

	scores := make([]Score, 0, len(scorers))
	for _, scorer := range scorers {
		scores = append(scores, scorer.Score(output, isErr, c.Expected))
	}

	r := Result{
		Case:     c,
		Scores:   scores,
		Output:   output,
		Error:    isErr,
		Duration: time.Since(start),
	}

	for _, rs := range resultScorers {
		r.Scores = append(r.Scores, rs.ScoreResult(r))
	}

	r.Passed = r.AverageScore() >= threshold
	return r
}

// extractText concatenates all text content from a CallToolResult.
func extractText(result *registry.CallToolResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, c := range result.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}
