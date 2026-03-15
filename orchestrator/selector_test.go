package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSelectBasicScoring(t *testing.T) {
	// Three stages with different "score" values embedded in data.
	stages := []StageFunc{
		func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"score": 0.3}}, nil
		},
		func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"score": 0.9}}, nil
		},
		func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"score": 0.5}}, nil
		},
	}

	scorer := func(out *StageOutput) float64 {
		if v, ok := out.Data["score"]; ok {
			if f, ok := v.(float64); ok {
				return f
			}
		}
		return 0
	}

	result, err := Select(context.Background(), stages, StageInput{}, SelectorConfig{Scorer: scorer})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pattern != "select" {
		t.Errorf("expected pattern=select, got %v", result.Pattern)
	}
	if result.StageCount != 3 {
		t.Errorf("expected StageCount=3, got %v", result.StageCount)
	}

	// Find the selected output (should be the one with score=0.9, which is outputs[1]).
	var selectedOut *StageOutput
	for _, out := range result.Outputs {
		if out != nil && out.Metadata != nil && out.Metadata["selected"] == "true" {
			selectedOut = out
			break
		}
	}

	if selectedOut == nil {
		t.Fatal("no output was marked as selected")
	}
	if selectedOut.Data["score"] != 0.9 {
		t.Errorf("expected selected output to have score=0.9, got %v", selectedOut.Data["score"])
	}
}

func TestSelectNilScorer(t *testing.T) {
	stages := []StageFunc{
		makeStage("a", 5*time.Millisecond, map[string]any{"x": 1}),
	}

	_, err := Select(context.Background(), stages, StageInput{}, SelectorConfig{Scorer: nil})
	if err == nil {
		t.Fatal("expected error for nil scorer, got nil")
	}
	if err.Error() != "orchestrator: scorer is required for Select" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSelectAllStagesError(t *testing.T) {
	stages := []StageFunc{
		func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return nil, errors.New("error1")
		},
		func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return nil, errors.New("error2")
		},
	}

	scorer := func(out *StageOutput) float64 { return 1.0 }

	result, err := Select(context.Background(), stages, StageInput{}, SelectorConfig{Scorer: scorer})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No output should be marked as selected.
	for i, out := range result.Outputs {
		if out != nil && out.Metadata != nil && out.Metadata["selected"] == "true" {
			t.Errorf("output[%d] should not be selected when all stages errored", i)
		}
	}
	// All outputs should have error status.
	for i, out := range result.Outputs {
		if out == nil {
			t.Errorf("output[%d] is nil", i)
			continue
		}
		if out.Status != "error" {
			t.Errorf("output[%d] expected error status, got %v", i, out.Status)
		}
	}
}

func TestSelectTimeout(t *testing.T) {
	stages := []StageFunc{
		makeStage("slow1", 500*time.Millisecond, map[string]any{"x": 1}),
		makeStage("slow2", 500*time.Millisecond, map[string]any{"y": 2}),
	}

	scorer := func(out *StageOutput) float64 { return 1.0 }

	start := time.Now()
	result, _ := Select(context.Background(), stages, StageInput{}, SelectorConfig{
		Scorer:  scorer,
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("timeout not enforced: elapsed %v (expected < 200ms)", elapsed)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSelectMaxConcurrency(t *testing.T) {
	// Verify MaxConcurrency is respected (uses same semaphore logic as FanOut).
	stages := []StageFunc{
		makeStage("a", 10*time.Millisecond, map[string]any{"v": 1.0}),
		makeStage("b", 10*time.Millisecond, map[string]any{"v": 2.0}),
		makeStage("c", 10*time.Millisecond, map[string]any{"v": 3.0}),
	}

	scorer := func(out *StageOutput) float64 {
		if v, ok := out.Data["v"].(float64); ok {
			return v
		}
		return 0
	}

	result, err := Select(context.Background(), stages, StageInput{}, SelectorConfig{
		Scorer:         scorer,
		MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Best should be the one with v=3.0.
	var selectedOut *StageOutput
	for _, out := range result.Outputs {
		if out != nil && out.Metadata != nil && out.Metadata["selected"] == "true" {
			selectedOut = out
			break
		}
	}
	if selectedOut == nil {
		t.Fatal("no output was selected")
	}
	if selectedOut.Data["v"] != 3.0 {
		t.Errorf("expected selected output v=3.0, got %v", selectedOut.Data["v"])
	}
}
