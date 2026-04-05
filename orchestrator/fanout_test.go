package orchestrator

import (
	"context"
	"errors"
	"maps"
	"sync/atomic"
	"testing"
	"time"
)

// makeStage creates a stage that waits for delay then returns data.
func makeStage(name string, delay time.Duration, data map[string]any) StageFunc {
	return func(ctx context.Context, input StageInput) (*StageOutput, error) {
		select {
		case <-time.After(delay):
			return &StageOutput{Data: data, Status: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// makeErrorStage creates a stage that returns an error after delay.
func makeErrorStage(delay time.Duration, errMsg string) StageFunc {
	return func(ctx context.Context, input StageInput) (*StageOutput, error) {
		select {
		case <-time.After(delay):
			return nil, errors.New(errMsg)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func TestFanOutBasic(t *testing.T) {
	stages := []StageFunc{
		makeStage("a", 10*time.Millisecond, map[string]any{"a": 1}),
		makeStage("b", 10*time.Millisecond, map[string]any{"b": 2}),
		makeStage("c", 10*time.Millisecond, map[string]any{"c": 3}),
	}

	result, err := FanOut(context.Background(), stages, StageInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pattern != "fan-out" {
		t.Errorf("expected pattern fan-out, got %v", result.Pattern)
	}
	if result.StageCount != 3 {
		t.Errorf("expected StageCount=3, got %v", result.StageCount)
	}
	if len(result.Outputs) != 3 {
		t.Errorf("expected 3 outputs, got %d", len(result.Outputs))
	}
	for i, out := range result.Outputs {
		if out == nil {
			t.Errorf("output[%d] is nil", i)
			continue
		}
		if out.Status != "ok" {
			t.Errorf("output[%d] status=%v, expected ok", i, out.Status)
		}
	}
}

func TestFanOutMaxConcurrency(t *testing.T) {
	// Track how many stages are running concurrently.
	var active int64
	var maxActive int64

	makeTrackedStage := func(delay time.Duration) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			cur := atomic.AddInt64(&active, 1)
			defer atomic.AddInt64(&active, -1)

			// Update max seen.
			for {
				old := atomic.LoadInt64(&maxActive)
				if cur <= old {
					break
				}
				if atomic.CompareAndSwapInt64(&maxActive, old, cur) {
					break
				}
			}

			select {
			case <-time.After(delay):
				return &StageOutput{Status: "ok"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	stages := []StageFunc{
		makeTrackedStage(20 * time.Millisecond),
		makeTrackedStage(20 * time.Millisecond),
		makeTrackedStage(20 * time.Millisecond),
		makeTrackedStage(20 * time.Millisecond),
	}

	_, err := FanOut(context.Background(), stages, StageInput{}, FanOutConfig{MaxConcurrency: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxActive > 2 {
		t.Errorf("expected max concurrency=2, got %d", maxActive)
	}
}

func TestFanOutFailFast(t *testing.T) {
	// One fast error stage, two slow stages.
	stages := []StageFunc{
		makeErrorStage(5*time.Millisecond, "fast error"),
		makeStage("slow1", 500*time.Millisecond, map[string]any{"x": 1}),
		makeStage("slow2", 500*time.Millisecond, map[string]any{"y": 2}),
	}

	start := time.Now()
	result, err := FanOut(context.Background(), stages, StageInput{}, FanOutConfig{FailFast: true})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from fail-fast, got nil")
	}
	if err.Error() != "fast error" {
		t.Errorf("expected 'fast error', got %v", err)
	}
	// Should complete much faster than 500ms since fail-fast cancels others.
	if elapsed > 200*time.Millisecond {
		t.Errorf("fail-fast took too long: %v (expected < 200ms)", elapsed)
	}
	_ = result
}

func TestFanOutMergeFunc(t *testing.T) {
	stages := []StageFunc{
		makeStage("a", 5*time.Millisecond, map[string]any{"a": 1}),
		makeStage("b", 5*time.Millisecond, map[string]any{"b": 2}),
		makeStage("c", 5*time.Millisecond, map[string]any{"c": 3}),
	}

	merge := func(outputs []*StageOutput) *StageOutput {
		combined := make(map[string]any)
		for _, out := range outputs {
			if out == nil {
				continue
			}
			maps.Copy(combined, out.Data)
		}
		return &StageOutput{Data: combined, Status: "ok"}
	}

	result, err := FanOut(context.Background(), stages, StageInput{}, FanOutConfig{MergeFunc: merge})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("expected 1 merged output, got %d", len(result.Outputs))
	}
	merged := result.Outputs[0]
	if merged.Data["a"] != 1 {
		t.Errorf("expected a=1, got %v", merged.Data["a"])
	}
	if merged.Data["b"] != 2 {
		t.Errorf("expected b=2, got %v", merged.Data["b"])
	}
	if merged.Data["c"] != 3 {
		t.Errorf("expected c=3, got %v", merged.Data["c"])
	}
}

func TestFanOutTimeout(t *testing.T) {
	stages := []StageFunc{
		makeStage("slow", 500*time.Millisecond, map[string]any{"x": 1}),
		makeStage("slow2", 500*time.Millisecond, map[string]any{"y": 2}),
	}

	start := time.Now()
	result, _ := FanOut(context.Background(), stages, StageInput{}, FanOutConfig{
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("timeout not enforced: elapsed %v (expected < 200ms)", elapsed)
	}
	if result == nil {
		t.Fatal("expected non-nil result even on timeout")
	}
	// All stages should have error status due to timeout.
	for i, out := range result.Outputs {
		if out == nil {
			t.Errorf("output[%d] is nil", i)
			continue
		}
		if out.Status != "error" {
			t.Errorf("output[%d] expected error status on timeout, got %v", i, out.Status)
		}
	}
}

func TestFanOutEmptyStages(t *testing.T) {
	result, err := FanOut(context.Background(), []StageFunc{}, StageInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.StageCount != 0 {
		t.Errorf("expected StageCount=0, got %v", result.StageCount)
	}
	if len(result.Outputs) != 0 {
		t.Errorf("expected 0 outputs, got %d", len(result.Outputs))
	}
}
