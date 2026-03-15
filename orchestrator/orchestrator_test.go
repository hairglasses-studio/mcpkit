package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestStageInputOutput(t *testing.T) {
	input := StageInput{
		Data:     map[string]any{"key": "value"},
		Previous: nil,
		Metadata: map[string]string{"trace": "abc"},
	}

	if input.Data["key"] != "value" {
		t.Errorf("expected Data[key]=value, got %v", input.Data["key"])
	}
	if input.Previous != nil {
		t.Error("expected Previous to be nil for first stage")
	}
	if input.Metadata["trace"] != "abc" {
		t.Errorf("expected Metadata[trace]=abc, got %v", input.Metadata["trace"])
	}
}

func TestStageOutputFields(t *testing.T) {
	out := &StageOutput{
		Data:     map[string]any{"result": 42},
		Status:   "ok",
		Duration: 5 * time.Millisecond,
		Metadata: map[string]string{"source": "test"},
	}

	if out.Status != "ok" {
		t.Errorf("expected status ok, got %v", out.Status)
	}
	if out.Error != "" {
		t.Errorf("expected empty error, got %v", out.Error)
	}
	if out.Data["result"] != 42 {
		t.Errorf("expected result=42, got %v", out.Data["result"])
	}
}

func TestOrchestratorResultFields(t *testing.T) {
	outputs := []*StageOutput{
		{Status: "ok", Data: map[string]any{"a": 1}},
		{Status: "ok", Data: map[string]any{"b": 2}},
	}
	result := &OrchestratorResult{
		Pattern:    "pipeline",
		Outputs:    outputs,
		Duration:   10 * time.Millisecond,
		StageCount: 2,
	}

	if result.Pattern != "pipeline" {
		t.Errorf("expected pattern=pipeline, got %v", result.Pattern)
	}
	if result.StageCount != 2 {
		t.Errorf("expected StageCount=2, got %v", result.StageCount)
	}
	if len(result.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(result.Outputs))
	}
}

func TestStageFuncSignature(t *testing.T) {
	// Verify StageFunc type is callable.
	stage := StageFunc(func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok", Data: input.Data}, nil
	})

	input := StageInput{Data: map[string]any{"x": 10}}
	out, err := stage(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "ok" {
		t.Errorf("expected status ok, got %v", out.Status)
	}
	if out.Data["x"] != 10 {
		t.Errorf("expected x=10, got %v", out.Data["x"])
	}
}
