package workflow

import (
	"context"
	"fmt"
	"testing"

	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestTracingMiddleware_Success(t *testing.T) {
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	node := func(ctx context.Context, state State) (State, error) {
		return Set(state, "done", true), nil
	}

	wrapped := mw("process", node)
	state, err := wrapped(context.Background(), NewState())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := Get[bool](state, "done")
	if !ok || !v {
		t.Fatal("expected done=true")
	}
}

func TestTracingMiddleware_Error(t *testing.T) {
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	node := func(ctx context.Context, state State) (State, error) {
		return state, fmt.Errorf("node failed")
	}

	wrapped := mw("fail-node", node)
	_, err := wrapped(context.Background(), NewState())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTracingMiddleware_WithEngine(t *testing.T) {
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	g := NewGraph()
	_ = g.AddNode("start", func(ctx context.Context, state State) (State, error) {
		return Set(state, "visited", true), nil
	})
	_ = g.AddEdge("start", EndNode)
	_ = g.SetStart("start")

	engine, err := NewEngine(g, EngineConfig{
		NodeMiddleware: []NodeMiddleware{TracingMiddleware(tracer)},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Run(context.Background(), "trace-test", NewState())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}
