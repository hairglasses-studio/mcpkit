package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestTracingMiddleware_Success(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "ok"}, nil
	}

	wrapped := mw("test-stage", stage)
	out, err := wrapped(context.Background(), StageInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "ok" {
		t.Fatalf("expected 'ok', got %q", out.Status)
	}
}

func TestTracingMiddleware_Error(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return nil, fmt.Errorf("stage failed")
	}

	wrapped := mw("fail-stage", stage)
	_, err := wrapped(context.Background(), StageInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTracingMiddleware_ErrorStatus(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	stage := func(ctx context.Context, input StageInput) (*StageOutput, error) {
		return &StageOutput{Status: "error", Error: "bad input"}, nil
	}

	wrapped := mw("err-stage", stage)
	out, err := wrapped(context.Background(), StageInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" {
		t.Fatalf("expected 'error', got %q", out.Status)
	}
}
