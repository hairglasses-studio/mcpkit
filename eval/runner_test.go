//go:build !official_sdk

package eval

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// testModule implements registry.ToolModule.
type testModule struct {
	tools []registry.ToolDefinition
}

func (m *testModule) Name() string                     { return "eval-test" }
func (m *testModule) Description() string              { return "test module" }
func (m *testModule) Tools() []registry.ToolDefinition { return m.tools }

func makeEchoTool(name, response string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name, Description: "echo tool"},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
			return registry.MakeTextResult(response), nil
		},
	}
}

func makeErrorTool(name, msg string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name, Description: "error tool"},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
			return registry.MakeErrorResult(msg), nil
		},
	}
}

func TestRun_AllPass(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeEchoTool("greet", "hello world"),
	}})

	suite := Suite{
		Name: "all-pass",
		Cases: []Case{
			{Name: "exact", Tool: "greet", Expected: "hello world"},
			{Name: "contains", Tool: "greet", Expected: "hello"},
		},
		Scorers: []Scorer{ExactMatch()},
		// only first case will get 1.0 on exact, second will get 0.0
	}

	// Override: use Contains for second case — but scorers are suite-wide.
	// Let's use Contains to make both pass:
	suite.Scorers = []Scorer{Contains()}

	summary := Run(context.Background(), suite, reg)

	if summary.Passed != 2 {
		t.Errorf("Passed = %d, want 2", summary.Passed)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", summary.Failed)
	}
	if summary.AvgScore != 1.0 {
		t.Errorf("AvgScore = %f, want 1.0", summary.AvgScore)
	}
}

func TestRun_PartialFail(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeEchoTool("greet", "hello"),
	}})

	suite := Suite{
		Name: "partial",
		Cases: []Case{
			{Name: "match", Tool: "greet", Expected: "hello"},
			{Name: "no-match", Tool: "greet", Expected: "goodbye"},
		},
		Scorers: []Scorer{ExactMatch()},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", summary.Failed)
	}
}

func TestRun_ToolNotFound(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	suite := Suite{
		Name: "not-found",
		Cases: []Case{
			{Name: "missing", Tool: "nonexistent", Expected: "x"},
		},
		Scorers: []Scorer{ExactMatch()},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Passed != 0 {
		t.Errorf("Passed = %d, want 0", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", summary.Failed)
	}
	if !summary.Results[0].Error {
		t.Error("expected Error=true for missing tool")
	}
}

func TestRun_Threshold(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeEchoTool("greet", "hello world"),
	}})

	suite := Suite{
		Name:      "threshold",
		Threshold: 0.5,
		Cases: []Case{
			{Name: "partial", Tool: "greet", Expected: "hello"},
		},
		Scorers: []Scorer{
			Contains(),   // 1.0 (contains "hello")
			ExactMatch(), // 0.0 (not exact "hello")
		},
	}

	summary := Run(context.Background(), suite, reg)

	// Average of 1.0 and 0.0 = 0.5, threshold is 0.5, so should pass
	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (avg=%.2f >= threshold=0.5)", summary.Passed, summary.AvgScore)
	}
}

func TestRun_MultipleScorers(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeErrorTool("fail_tool", "something went wrong"),
	}})

	suite := Suite{
		Name: "multi-scorer",
		Cases: []Case{
			{Name: "check-error", Tool: "fail_tool", Expected: "wrong"},
		},
		Scorers: []Scorer{
			IsError(true),
			Contains(),
		},
	}

	summary := Run(context.Background(), suite, reg)

	if len(summary.Results[0].Scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(summary.Results[0].Scores))
	}
	for _, s := range summary.Results[0].Scores {
		if s.Value != 1.0 {
			t.Errorf("scorer %s: value = %f, want 1.0", s.Scorer, s.Value)
		}
	}
}

func TestRun_WithResultScorers(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeEchoTool("greet", "hello world"),
	}})

	suite := Suite{
		Name:      "result-scorers",
		Threshold: 1.0,
		Cases: []Case{
			{Name: "success", Tool: "greet", Expected: "hello"},
		},
		Scorers:       []Scorer{Contains()},
		ResultScorers: []ResultScorer{ErrorRate(), Latency(1000 * time.Millisecond)},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Total != 1 {
		t.Fatalf("Total = %d, want 1", summary.Total)
	}

	r := summary.Results[0]
	// Expect 3 scores: 1 from Scorers (Contains) + 2 from ResultScorers (ErrorRate, Latency)
	if len(r.Scores) != 3 {
		t.Fatalf("got %d scores, want 3", len(r.Scores))
	}

	scoresByName := make(map[string]Score)
	for _, s := range r.Scores {
		scoresByName[s.Scorer] = s
	}

	if s, ok := scoresByName["contains"]; !ok {
		t.Error("missing 'contains' scorer")
	} else if s.Value != 1.0 {
		t.Errorf("contains: value = %f, want 1.0", s.Value)
	}

	if s, ok := scoresByName["error_rate"]; !ok {
		t.Error("missing 'error_rate' scorer")
	} else if s.Value != 1.0 {
		t.Errorf("error_rate: value = %f, want 1.0 (no error expected)", s.Value)
	}

	if s, ok := scoresByName["latency"]; !ok {
		t.Error("missing 'latency' scorer")
	} else if s.Value != 1.0 {
		t.Errorf("latency: value = %f, want 1.0", s.Value)
	}

	if !r.Passed {
		t.Error("expected case to pass")
	}
}

func TestRun_WithResultScorers_ErrorTool(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeErrorTool("fail_tool", "something went wrong"),
	}})

	suite := Suite{
		Name:      "result-scorers-error",
		Threshold: 0.5,
		Cases: []Case{
			{Name: "error-case", Tool: "fail_tool", Expected: nil},
		},
		Scorers:       []Scorer{},
		ResultScorers: []ResultScorer{ErrorRate()},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Total != 1 {
		t.Fatalf("Total = %d, want 1", summary.Total)
	}

	r := summary.Results[0]
	if len(r.Scores) != 1 {
		t.Fatalf("got %d scores, want 1", len(r.Scores))
	}

	s := r.Scores[0]
	if s.Scorer != "error_rate" {
		t.Errorf("scorer = %q, want error_rate", s.Scorer)
	}
	if s.Value != 0.0 {
		t.Errorf("error_rate value = %f, want 0.0 for error tool", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason for error tool")
	}
}

func TestRun_EmptySuite(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	suite := Suite{Name: "empty"}

	summary := Run(context.Background(), suite, reg)

	if summary.Total != 0 {
		t.Errorf("Total = %d, want 0", summary.Total)
	}
	if summary.PassRate() != 0 {
		t.Errorf("PassRate = %f, want 0", summary.PassRate())
	}
}
