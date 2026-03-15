package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCompensationStack_LIFO verifies that Push and Compensate operate in
// last-in-first-out order.
func TestCompensationStack_LIFO(t *testing.T) {
	var order []string
	var mu sync.Mutex

	cs := NewCompensationStack()

	for _, name := range []string{"a", "b", "c"} {
		n := name // capture
		cs.Push(CompensationRecord{
			NodeName: n,
			Compensate: func(_ context.Context, _ State) error {
				mu.Lock()
				order = append(order, n)
				mu.Unlock()
				return nil
			},
			State: NewState(),
		})
	}

	if cs.Len() != 3 {
		t.Fatalf("Len = %d; want 3", cs.Len())
	}

	errs := cs.Compensate(context.Background())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"c", "b", "a"}
	if len(order) != len(want) {
		t.Fatalf("order len = %d; want %d: %v", len(order), len(want), order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("order[%d] = %q; want %q", i, order[i], w)
		}
	}
}

// TestCompensableNode_NoFailure verifies that when all compensable nodes succeed,
// no compensation runs (stack is populated but never triggered).
func TestCompensableNode_NoFailure(t *testing.T) {
	var compensated []string
	var mu sync.Mutex

	makeCompensate := func(name string) CompensateFunc {
		return func(_ context.Context, _ State) error {
			mu.Lock()
			compensated = append(compensated, name)
			mu.Unlock()
			return nil
		}
	}

	g := NewGraph()
	if err := g.AddCompensableNode("a", func(_ context.Context, s State) (State, error) {
		return Set(s, "a", true), nil
	}, makeCompensate("a")); err != nil {
		t.Fatalf("AddCompensableNode a: %v", err)
	}
	if err := g.AddCompensableNode("b", func(_ context.Context, s State) (State, error) {
		return Set(s, "b", true), nil
	}, makeCompensate("b")); err != nil {
		t.Fatalf("AddCompensableNode b: %v", err)
	}
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:            10,
		DefaultNodeTimeout:  time.Second,
		CompensateOnFailure: true,
	})
	result, err := e.Run(context.Background(), "no-failure", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(compensated) != 0 {
		t.Errorf("expected no compensations on success, got: %v", compensated)
	}
}

// TestCompensableNode_FailureTriggers verifies that when node C fails, the
// compensations for B then A run in reverse (LIFO) order.
func TestCompensableNode_FailureTriggers(t *testing.T) {
	var compensated []string
	var mu sync.Mutex

	makeCompensate := func(name string) CompensateFunc {
		return func(_ context.Context, _ State) error {
			mu.Lock()
			compensated = append(compensated, name)
			mu.Unlock()
			return nil
		}
	}

	g := NewGraph()
	if err := g.AddCompensableNode("a", func(_ context.Context, s State) (State, error) {
		return Set(s, "a", true), nil
	}, makeCompensate("a")); err != nil {
		t.Fatalf("AddCompensableNode a: %v", err)
	}
	if err := g.AddCompensableNode("b", func(_ context.Context, s State) (State, error) {
		return Set(s, "b", true), nil
	}, makeCompensate("b")); err != nil {
		t.Fatalf("AddCompensableNode b: %v", err)
	}
	mustAddNode(t, g, "c", func(_ context.Context, s State) (State, error) {
		return s, errors.New("c failed")
	})
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:            10,
		DefaultNodeTimeout:  time.Second,
		CompensateOnFailure: true,
	})
	result, err := e.Run(context.Background(), "c-fails", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"b", "a"}
	if len(compensated) != len(want) {
		t.Fatalf("compensated = %v; want %v", compensated, want)
	}
	for i, w := range want {
		if compensated[i] != w {
			t.Errorf("compensated[%d] = %q; want %q", i, compensated[i], w)
		}
	}
}

// TestCompensableNode_PartialCompensation verifies that when one compensation
// returns an error, the others still run and the overall run still returns
// RunStatusFailed (errors are collected but don't abort compensation).
func TestCompensableNode_PartialCompensation(t *testing.T) {
	var compensated []string
	var mu sync.Mutex

	g := NewGraph()
	// a: compensate succeeds
	if err := g.AddCompensableNode("a", func(_ context.Context, s State) (State, error) {
		return Set(s, "a", true), nil
	}, func(_ context.Context, _ State) error {
		mu.Lock()
		compensated = append(compensated, "a")
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("AddCompensableNode a: %v", err)
	}
	// b: compensate fails
	if err := g.AddCompensableNode("b", func(_ context.Context, s State) (State, error) {
		return Set(s, "b", true), nil
	}, func(_ context.Context, _ State) error {
		mu.Lock()
		compensated = append(compensated, "b")
		mu.Unlock()
		return fmt.Errorf("b compensate error")
	}); err != nil {
		t.Fatalf("AddCompensableNode b: %v", err)
	}
	// c: forward fails, triggering compensation
	mustAddNode(t, g, "c", func(_ context.Context, s State) (State, error) {
		return s, errors.New("c failed")
	})
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:            10,
		DefaultNodeTimeout:  time.Second,
		CompensateOnFailure: true,
	})
	result, err := e.Run(context.Background(), "partial-comp", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()
	// Both b and a should have been attempted (LIFO: b first, then a).
	if len(compensated) != 2 {
		t.Fatalf("compensated = %v; want [b a]", compensated)
	}
	if compensated[0] != "b" || compensated[1] != "a" {
		t.Errorf("compensated order = %v; want [b a]", compensated)
	}
}

// TestCompensableNode_DisabledByDefault verifies that when CompensateOnFailure
// is false (the default), no compensation runs even if compensable nodes exist.
func TestCompensableNode_DisabledByDefault(t *testing.T) {
	var compensated []string
	var mu sync.Mutex

	makeCompensate := func(name string) CompensateFunc {
		return func(_ context.Context, _ State) error {
			mu.Lock()
			compensated = append(compensated, name)
			mu.Unlock()
			return nil
		}
	}

	g := NewGraph()
	if err := g.AddCompensableNode("a", func(_ context.Context, s State) (State, error) {
		return Set(s, "a", true), nil
	}, makeCompensate("a")); err != nil {
		t.Fatalf("AddCompensableNode a: %v", err)
	}
	mustAddNode(t, g, "b", func(_ context.Context, s State) (State, error) {
		return s, errors.New("b failed")
	})
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", EndNode)
	mustSetStart(t, g, "a")

	// CompensateOnFailure is NOT set (defaults to false).
	e := newEngine(t, g, EngineConfig{
		MaxSteps:           10,
		DefaultNodeTimeout: time.Second,
	})
	result, err := e.Run(context.Background(), "disabled", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(compensated) != 0 {
		t.Errorf("expected no compensations when disabled, got: %v", compensated)
	}
}

// TestCompensableNode_WithHooks verifies that OnCompensationStart and
// OnCompensationEnd hooks fire for each compensated node.
func TestCompensableNode_WithHooks(t *testing.T) {
	var startHooks []string
	var endHooks []string
	var hookErrs []error
	var mu sync.Mutex

	makeCompensate := func(name string) CompensateFunc {
		return func(_ context.Context, _ State) error {
			return nil
		}
	}

	g := NewGraph()
	if err := g.AddCompensableNode("a", func(_ context.Context, s State) (State, error) {
		return Set(s, "a", true), nil
	}, makeCompensate("a")); err != nil {
		t.Fatalf("AddCompensableNode a: %v", err)
	}
	if err := g.AddCompensableNode("b", func(_ context.Context, s State) (State, error) {
		return Set(s, "b", true), nil
	}, makeCompensate("b")); err != nil {
		t.Fatalf("AddCompensableNode b: %v", err)
	}
	mustAddNode(t, g, "c", func(_ context.Context, s State) (State, error) {
		return s, errors.New("c failed")
	})
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", EndNode)
	mustSetStart(t, g, "a")

	hooks := Hooks{
		OnCompensationStart: func(nodeName string) {
			mu.Lock()
			startHooks = append(startHooks, nodeName)
			mu.Unlock()
		},
		OnCompensationEnd: func(nodeName string, err error) {
			mu.Lock()
			endHooks = append(endHooks, nodeName)
			hookErrs = append(hookErrs, err)
			mu.Unlock()
		},
	}

	e := newEngine(t, g, EngineConfig{
		MaxSteps:            10,
		DefaultNodeTimeout:  time.Second,
		CompensateOnFailure: true,
		Hooks:               hooks,
	})
	result, err := e.Run(context.Background(), "hooks-run", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()

	// LIFO: b compensated first, then a.
	wantOrder := []string{"b", "a"}
	if len(startHooks) != 2 {
		t.Fatalf("OnCompensationStart called %d times; want 2", len(startHooks))
	}
	if len(endHooks) != 2 {
		t.Fatalf("OnCompensationEnd called %d times; want 2", len(endHooks))
	}
	for i, w := range wantOrder {
		if startHooks[i] != w {
			t.Errorf("startHooks[%d] = %q; want %q", i, startHooks[i], w)
		}
		if endHooks[i] != w {
			t.Errorf("endHooks[%d] = %q; want %q", i, endHooks[i], w)
		}
		if hookErrs[i] != nil {
			t.Errorf("hookErrs[%d] = %v; want nil", i, hookErrs[i])
		}
	}
}

// TestAddCompensableNode_NilCompensate verifies that passing a nil compensate
// function returns an error.
func TestAddCompensableNode_NilCompensate(t *testing.T) {
	g := NewGraph()
	err := g.AddCompensableNode("a", noop, nil)
	if err == nil {
		t.Error("expected error for nil compensate function")
	}
}

// TestCompensationStack_EmptyCompensate verifies that compensating an empty
// stack is a no-op with no errors.
func TestCompensationStack_EmptyCompensate(t *testing.T) {
	cs := NewCompensationStack()
	errs := cs.Compensate(context.Background())
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty stack, got %v", errs)
	}
}

// TestCompensationStack_ErrorsCollected verifies that Compensate collects
// errors from all failing compensations.
func TestCompensationStack_ErrorsCollected(t *testing.T) {
	cs := NewCompensationStack()
	errA := fmt.Errorf("error from a")
	errB := fmt.Errorf("error from b")

	cs.Push(CompensationRecord{
		NodeName: "a",
		Compensate: func(_ context.Context, _ State) error { return errA },
		State:    NewState(),
	})
	cs.Push(CompensationRecord{
		NodeName: "b",
		Compensate: func(_ context.Context, _ State) error { return errB },
		State:    NewState(),
	})

	errs := cs.Compensate(context.Background())
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}
