package orchestrator

import (
	"context"
	"errors"
	"testing"
)

func TestSelectDynamicPatternOverride(t *testing.T) {
	t.Parallel()

	decision, err := SelectDynamicPattern(StageInput{Metadata: map[string]string{
		"orchestrator.pattern": "select",
	}}, 3)
	if err != nil {
		t.Fatalf("SelectDynamicPattern: %v", err)
	}
	if decision.Pattern != DynamicPatternSelect || decision.Reason != "metadata override" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestSelectDynamicPatternUnknownOverride(t *testing.T) {
	t.Parallel()

	_, err := SelectDynamicPattern(StageInput{Metadata: map[string]string{
		"orchestrator.pattern": "swarm",
	}}, 3)
	if !errors.Is(err, ErrUnknownDynamicPattern) {
		t.Fatalf("error = %v, want ErrUnknownDynamicPattern", err)
	}
}

func TestSelectDynamicPatternMetadataRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		metadata   map[string]string
		stageCount int
		want       DynamicPattern
	}{
		{name: "best-of", metadata: map[string]string{"orchestrator.best_of": "true"}, stageCount: 3, want: DynamicPatternSelect},
		{name: "ordered", metadata: map[string]string{"orchestrator.ordered": "yes"}, stageCount: 3, want: DynamicPatternPipeline},
		{name: "parallel false", metadata: map[string]string{"orchestrator.parallel": "false"}, stageCount: 3, want: DynamicPatternPipeline},
		{name: "parallel true", metadata: map[string]string{"orchestrator.parallel": "true"}, stageCount: 1, want: DynamicPatternFanOut},
		{name: "multi default", metadata: nil, stageCount: 2, want: DynamicPatternFanOut},
		{name: "single default", metadata: nil, stageCount: 1, want: DynamicPatternPipeline},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			decision, err := SelectDynamicPattern(StageInput{Metadata: tc.metadata}, tc.stageCount)
			if err != nil {
				t.Fatalf("SelectDynamicPattern: %v", err)
			}
			if decision.Pattern != tc.want {
				t.Fatalf("pattern = %q, want %q", decision.Pattern, tc.want)
			}
		})
	}
}

func TestRunDynamicPatternPipeline(t *testing.T) {
	t.Parallel()

	stages := []StageFunc{
		func(_ context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"n": input.Data["n"].(int) + 1}}, nil
		},
		func(_ context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"n": input.Data["n"].(int) * 2}}, nil
		},
	}
	result, decision, err := RunDynamicPattern(context.Background(), stages, StageInput{
		Data:     map[string]any{"n": 2},
		Metadata: map[string]string{"orchestrator.ordered": "true"},
	}, DynamicPatternConfig{})
	if err != nil {
		t.Fatalf("RunDynamicPattern: %v", err)
	}
	if decision.Pattern != DynamicPatternPipeline || result.Pattern != "pipeline" {
		t.Fatalf("unexpected pattern: decision=%+v result=%+v", decision, result)
	}
	if got := result.Outputs[1].Data["n"]; got != 6 {
		t.Fatalf("final n = %v, want 6", got)
	}
}

func TestRunDynamicPatternFanOut(t *testing.T) {
	t.Parallel()

	stages := []StageFunc{
		func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"a": 1}}, nil
		},
		func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"b": 2}}, nil
		},
	}
	result, decision, err := RunDynamicPattern(context.Background(), stages, StageInput{}, DynamicPatternConfig{})
	if err != nil {
		t.Fatalf("RunDynamicPattern: %v", err)
	}
	if decision.Pattern != DynamicPatternFanOut || result.Pattern != "fan-out" {
		t.Fatalf("unexpected pattern: decision=%+v result=%+v", decision, result)
	}
}

func TestRunDynamicPatternSelect(t *testing.T) {
	t.Parallel()

	stages := []StageFunc{
		func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"score": 1}}, nil
		},
		func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"score": 5}}, nil
		},
	}
	result, decision, err := RunDynamicPattern(context.Background(), stages, StageInput{
		Metadata: map[string]string{"orchestrator.best_of": "true"},
	}, DynamicPatternConfig{Selector: SelectorConfig{
		Scorer: func(out *StageOutput) float64 {
			return float64(out.Data["score"].(int))
		},
	}})
	if err != nil {
		t.Fatalf("RunDynamicPattern: %v", err)
	}
	if decision.Pattern != DynamicPatternSelect || result.Pattern != "select" {
		t.Fatalf("unexpected pattern: decision=%+v result=%+v", decision, result)
	}
	if result.Outputs[1].Metadata["selected"] != "true" {
		t.Fatalf("second output was not selected: %+v", result.Outputs)
	}
}
