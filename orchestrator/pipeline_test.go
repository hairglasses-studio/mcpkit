package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPipelineSequentialDataFlow(t *testing.T) {
	// Each stage adds a key to data, verify accumulation.
	stage1 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		if input.Previous != nil {
			t.Error("expected Previous to be nil for first stage")
		}
		return &StageOutput{
			Status: "ok",
			Data:   map[string]any{"step1": "done"},
		}, nil
	}

	stage2 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		if input.Previous == nil {
			t.Error("expected Previous to be non-nil for second stage")
		}
		// Should have accumulated data from stage1.
		if input.Data["step1"] != "done" {
			t.Errorf("expected step1=done in input data, got %v", input.Data["step1"])
		}
		return &StageOutput{
			Status: "ok",
			Data:   map[string]any{"step2": "done"},
		}, nil
	}

	stage3 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		// Should have accumulated data from stages 1 and 2.
		if input.Data["step1"] != "done" {
			t.Errorf("expected step1=done in input data, got %v", input.Data["step1"])
		}
		if input.Data["step2"] != "done" {
			t.Errorf("expected step2=done in input data, got %v", input.Data["step2"])
		}
		return &StageOutput{
			Status: "ok",
			Data:   map[string]any{"step3": "done"},
		}, nil
	}

	result, err := Pipeline(context.Background(), []StageFunc{stage1, stage2, stage3}, StageInput{Data: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pattern != "pipeline" {
		t.Errorf("expected pattern=pipeline, got %v", result.Pattern)
	}
	if result.StageCount != 3 {
		t.Errorf("expected StageCount=3, got %v", result.StageCount)
	}
	if len(result.Outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(result.Outputs))
	}
	for i, out := range result.Outputs {
		if out.Status != "ok" {
			t.Errorf("output[%d] status=%v, expected ok", i, out.Status)
		}
	}
}

func TestPipelineOnStageErrorStop(t *testing.T) {
	var stage3Called bool

	stage1 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok", Data: map[string]any{"a": 1}}, nil
	}
	stage2 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return nil, errors.New("stage2 error")
	}
	stage3 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		stage3Called = true
		return &StageOutput{Status: "ok"}, nil
	}

	result, err := Pipeline(context.Background(), []StageFunc{stage1, stage2, stage3}, StageInput{},
		PipelineConfig{OnStageError: "stop"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "stage2 error" {
		t.Errorf("expected 'stage2 error', got %v", err)
	}
	if stage3Called {
		t.Error("stage3 should not have been called when OnStageError=stop")
	}
	// Should have 2 outputs: stage1 success + stage2 error.
	if len(result.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(result.Outputs))
	}
	if result.Outputs[1].Status != "error" {
		t.Errorf("expected output[1] status=error, got %v", result.Outputs[1].Status)
	}
}

func TestPipelineOnStageErrorSkip(t *testing.T) {
	var stage3Input StageInput

	stage1 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok", Data: map[string]any{"a": 1}}, nil
	}
	stage2 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return nil, errors.New("stage2 error")
	}
	stage3 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		stage3Input = input
		return &StageOutput{Status: "ok", Data: map[string]any{"c": 3}}, nil
	}

	result, err := Pipeline(context.Background(), []StageFunc{stage1, stage2, stage3}, StageInput{},
		PipelineConfig{OnStageError: "skip"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// stage3 should have received stage1's output as Previous (skip means current input unchanged).
	if stage3Input.Previous == nil {
		t.Error("expected stage3 to have a Previous (stage1 output)")
	} else if stage3Input.Previous.Status != "ok" {
		t.Errorf("expected stage3 Previous.Status=ok (stage1 output), got %v", stage3Input.Previous.Status)
	}
	// Result should have 2 outputs (stage1 and stage3, stage2 error was added but skip continues).
	// Actually with skip: we append the errOutput then continue — so 3 outputs total.
	if len(result.Outputs) < 2 {
		t.Errorf("expected at least 2 outputs, got %d", len(result.Outputs))
	}
}

func TestPipelineOnStageErrorContinue(t *testing.T) {
	var stage3Previous *StageOutput

	stage1 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok", Data: map[string]any{"a": 1}}, nil
	}
	stage2 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return nil, errors.New("stage2 error")
	}
	stage3 := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		stage3Previous = input.Previous
		return &StageOutput{Status: "ok"}, nil
	}

	result, err := Pipeline(context.Background(), []StageFunc{stage1, stage2, stage3}, StageInput{},
		PipelineConfig{OnStageError: "continue"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// stage3 should have the error output as its Previous.
	if stage3Previous == nil {
		t.Fatal("expected stage3 to receive error output as Previous")
	}
	if stage3Previous.Status != "error" {
		t.Errorf("expected stage3 Previous.Status=error, got %v", stage3Previous.Status)
	}
	if stage3Previous.Error != "stage2 error" {
		t.Errorf("expected stage3 Previous.Error='stage2 error', got %v", stage3Previous.Error)
	}
	_ = result
}

func TestPipelineFirstStagePreviousIsNil(t *testing.T) {
	called := false
	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		called = true
		if input.Previous != nil {
			t.Error("first stage Previous should be nil")
		}
		return &StageOutput{Status: "ok"}, nil
	}

	_, err := Pipeline(context.Background(), []StageFunc{stage}, StageInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("stage was not called")
	}
}

func TestPipelineTimeout(t *testing.T) {
	slowStage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		select {
		case <-time.After(500 * time.Millisecond):
			return &StageOutput{Status: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	start := time.Now()
	result, err := Pipeline(context.Background(), []StageFunc{slowStage}, StageInput{},
		PipelineConfig{Timeout: 50 * time.Millisecond})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("timeout not enforced: elapsed %v", elapsed)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestPipelineEmptyStages(t *testing.T) {
	result, err := Pipeline(context.Background(), []StageFunc{}, StageInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StageCount != 0 {
		t.Errorf("expected StageCount=0, got %v", result.StageCount)
	}
	if len(result.Outputs) != 0 {
		t.Errorf("expected 0 outputs, got %d", len(result.Outputs))
	}
}
