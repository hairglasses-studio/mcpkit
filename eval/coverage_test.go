//go:build !official_sdk

package eval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// makeGoErrorTool returns a tool whose handler returns a non-nil Go error.
func makeGoErrorTool(name string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name, Description: "go error tool"},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
			return nil, errors.New("handler go error")
		},
	}
}

// makeNilResultTool returns a tool whose handler returns (nil, nil).
func makeNilResultTool(name string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name, Description: "nil result tool"},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
			return nil, nil
		},
	}
}

// TestRunCase_HandlerGoError covers the err != nil branch in runCase (line 106-108).
func TestRunCase_HandlerGoError(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeGoErrorTool("go_err"),
	}})

	suite := Suite{
		Name: "go-error",
		Cases: []Case{
			{Name: "go-err", Tool: "go_err", Expected: "handler go error"},
		},
		Scorers: []Scorer{Contains()},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Total != 1 {
		t.Fatalf("Total = %d, want 1", summary.Total)
	}
	r := summary.Results[0]
	// Error flag should be set because handler returned a Go error.
	if !r.Error {
		t.Error("expected Error=true when handler returns Go error")
	}
	// Output should contain the error message.
	if r.Output != "handler go error" {
		t.Errorf("Output = %q, want %q", r.Output, "handler go error")
	}
}

// TestRunCase_NilResult covers the nil result path in runCase and extractText nil guard.
func TestRunCase_NilResult(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeNilResultTool("nil_result"),
	}})

	suite := Suite{
		Name: "nil-result",
		Cases: []Case{
			{Name: "nil-case", Tool: "nil_result", Expected: ""},
		},
		Scorers: []Scorer{ExactMatch()},
	}

	summary := Run(context.Background(), suite, reg)

	if summary.Total != 1 {
		t.Fatalf("Total = %d, want 1", summary.Total)
	}
	r := summary.Results[0]
	if r.Error {
		t.Error("expected Error=false for nil result (no go error)")
	}
	if r.Output != "" {
		t.Errorf("Output = %q, want empty for nil result", r.Output)
	}
	// ExactMatch on empty output with empty expected should pass.
	if !r.Passed {
		t.Error("expected Passed=true: empty output matches empty expected")
	}
}

// TestExtractText_NilResult covers the nil guard in extractText directly.
func TestExtractText_NilResult(t *testing.T) {
	t.Parallel()
	got := extractText(nil)
	if got != "" {
		t.Errorf("extractText(nil) = %q, want empty", got)
	}
}

// TestJSONPath_NonObjectInPath covers the "not an object at" branch (line 121).
func TestJSONPath_NonObjectInPath(t *testing.T) {
	t.Parallel()
	// "a" is a string, so navigating "a.b" will fail at "b" because current is not a map.
	s := JSONPath("a.b").Score(`{"a": "string_not_object"}`, false, "x")
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason for non-object traversal")
	}
}

// TestCustomScorer_Name covers the Name() method on customScorer (line 200).
func TestCustomScorer_Name(t *testing.T) {
	t.Parallel()
	scorer := Custom("my_custom", func(output string, isError bool, expected interface{}) Score {
		return Score{Value: 1.0}
	})
	if scorer.Name() != "my_custom" {
		t.Errorf("Name() = %q, want %q", scorer.Name(), "my_custom")
	}
}

// TestRunT_AllPass covers the RunT path when all cases pass.
func TestRunT_AllPass(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{
		makeEchoTool("greet_runt", "hello"),
	}})

	suite := Suite{
		Name: "runt-pass",
		Cases: []Case{
			{Name: "pass", Tool: "greet_runt", Expected: "hello"},
		},
		Scorers: []Scorer{ExactMatch()},
	}

	summary := RunT(t, context.Background(), suite, reg)
	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1", summary.Passed)
	}
}

// TestRunCase_WithArgs covers the c.Args != nil branch in runCase.
func TestRunCase_WithArgs(t *testing.T) {
	t.Parallel()

	var capturedArgs map[string]interface{}
	argCaptureTool := registry.ToolDefinition{
		Tool: registry.Tool{Name: "arg_capture", Description: "captures args"},
		Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			args := registry.ExtractArguments(req)
			val, _ := args["key"].(string)
			return registry.MakeTextResult("got:" + val), nil
		},
	}
	_ = capturedArgs

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{tools: []registry.ToolDefinition{argCaptureTool}})

	suite := Suite{
		Name: "with-args",
		Cases: []Case{
			{
				Name:     "args-case",
				Tool:     "arg_capture",
				Args:     map[string]interface{}{"key": "value"},
				Expected: "got:value",
			},
		},
		Scorers: []Scorer{ExactMatch()},
	}

	summary := Run(context.Background(), suite, reg)
	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (output: %q)", summary.Passed, summary.Results[0].Output)
	}
}

// TestJSONPath_InvalidJSON covers the json.Unmarshal error branch in JSONPath.Score.
func TestJSONPath_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := JSONPath("key").Score(`not valid json`, false, "x")
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0 for invalid JSON", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason for invalid JSON")
	}
}

// TestJSONPath_ValueMismatch covers the value != expected branch at the end of JSONPath.Score.
func TestJSONPath_ValueMismatch(t *testing.T) {
	t.Parallel()
	s := JSONPath("count").Score(`{"count": 5}`, false, float64(99))
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0 for value mismatch", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason for value mismatch")
	}
}

// TestLatency_ExactLimit covers the boundary where duration == max (should pass).
func TestLatency_ExactLimit(t *testing.T) {
	t.Parallel()
	scorer := Latency(100 * time.Millisecond)
	s := scorer.ScoreResult(Result{Duration: 100 * time.Millisecond})
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0 at exact limit", s.Value)
	}
}

// TestLatency_ZeroMax covers zero max duration: any positive duration should fail.
func TestLatency_ZeroMax(t *testing.T) {
	t.Parallel()
	scorer := Latency(0)
	s := scorer.ScoreResult(Result{Duration: 1 * time.Nanosecond})
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0 when duration exceeds zero max", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason when exceeding zero max")
	}
}

// TestLatency_Name covers the Name() method on latencyScorer.
func TestLatency_Name(t *testing.T) {
	t.Parallel()
	scorer := Latency(1 * time.Second)
	if scorer.Name() != "latency" {
		t.Errorf("Name() = %q, want latency", scorer.Name())
	}
}

// TestErrorRate_Name covers the Name() method on errorRateScorer.
func TestErrorRate_Name(t *testing.T) {
	t.Parallel()
	scorer := ErrorRate()
	if scorer.Name() != "error_rate" {
		t.Errorf("Name() = %q, want error_rate", scorer.Name())
	}
}
