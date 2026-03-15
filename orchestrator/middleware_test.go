package orchestrator

import (
	"context"
	"testing"
)

func TestWrapStage_Single(t *testing.T) {
	var log []string

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		log = append(log, "stage")
		return &StageOutput{Status: "ok"}, nil
	}

	mw := func(name string, next StageFunc) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			log = append(log, "before:"+name)
			out, err := next(ctx, input)
			log = append(log, "after:"+name)
			return out, err
		}
	}

	wrapped := WrapStage(stage, "test-stage", mw)
	out, err := wrapped(context.Background(), StageInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", out.Status)
	}
	if len(log) != 3 {
		t.Fatalf("expected 3 log entries, got %d: %v", len(log), log)
	}
	if log[0] != "before:test-stage" || log[1] != "stage" || log[2] != "after:test-stage" {
		t.Fatalf("unexpected log order: %v", log)
	}
}

func TestWrapStage_MultipleMiddleware(t *testing.T) {
	var log []string

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		log = append(log, "stage")
		return &StageOutput{Status: "ok"}, nil
	}

	makeMW := func(id string) StageMiddleware {
		return func(name string, next StageFunc) StageFunc {
			return func(ctx context.Context, input StageInput) (*StageOutput, error) {
				log = append(log, id+":before")
				out, err := next(ctx, input)
				log = append(log, id+":after")
				return out, err
			}
		}
	}

	wrapped := WrapStage(stage, "s", makeMW("A"), makeMW("B"))
	_, err := wrapped(context.Background(), StageInput{})
	if err != nil {
		t.Fatal(err)
	}

	// A is outermost, B is innermost
	expected := []string{"A:before", "B:before", "stage", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], e)
		}
	}
}

func TestWrapStages(t *testing.T) {
	var names []string

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok"}, nil
	}

	mw := func(name string, next StageFunc) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			names = append(names, name)
			return next(ctx, input)
		}
	}

	stages := []StageFunc{stage, stage, stage}
	stageNames := []string{"alpha", "beta"} // third gets default name

	wrapped := WrapStages(stages, stageNames, mw)
	if len(wrapped) != 3 {
		t.Fatalf("expected 3 wrapped stages, got %d", len(wrapped))
	}

	for _, w := range wrapped {
		_, err := w(context.Background(), StageInput{})
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}
	if names[0] != "alpha" || names[1] != "beta" || names[2] != "stage-2" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestWrapStages_NoMiddleware(t *testing.T) {
	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok"}, nil
	}
	stages := []StageFunc{stage}
	wrapped := WrapStages(stages, nil)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(wrapped))
	}
	out, err := wrapped[0](context.Background(), StageInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "ok" {
		t.Fatalf("expected 'ok', got %q", out.Status)
	}
}

type tenantKey struct{}

func TestWrapStage_ContextPropagation(t *testing.T) {
	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		tenant, ok := ctx.Value(tenantKey{}).(string)
		if !ok || tenant != "acme" {
			t.Fatalf("expected tenant 'acme', got %q", tenant)
		}
		return &StageOutput{Status: "ok"}, nil
	}

	mw := func(name string, next StageFunc) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			_, ok := ctx.Value(tenantKey{}).(string)
			if !ok {
				t.Fatal("tenant not in context")
			}
			return next(ctx, input)
		}
	}

	wrapped := WrapStage(stage, "s", mw)
	ctx := context.WithValue(context.Background(), tenantKey{}, "acme")
	_, err := wrapped(ctx, StageInput{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWrapStage_ErrorPropagation(t *testing.T) {
	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return nil, context.DeadlineExceeded
	}

	mw := func(name string, next StageFunc) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			return next(ctx, input)
		}
	}

	wrapped := WrapStage(stage, "s", mw)
	_, err := wrapped(context.Background(), StageInput{})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}
