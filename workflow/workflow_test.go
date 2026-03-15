package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- helpers ---

func noop(_ context.Context, s State) (State, error) { return s, nil }

func mustAddNode(t *testing.T, g *Graph, name string, fn NodeFunc, opts ...NodeOption) {
	t.Helper()
	if err := g.AddNode(name, fn, opts...); err != nil {
		t.Fatalf("AddNode(%q): %v", name, err)
	}
}

func mustAddEdge(t *testing.T, g *Graph, from, to string) {
	t.Helper()
	if err := g.AddEdge(from, to); err != nil {
		t.Fatalf("AddEdge(%q->%q): %v", from, to, err)
	}
}

func mustSetStart(t *testing.T, g *Graph, name string) {
	t.Helper()
	if err := g.SetStart(name); err != nil {
		t.Fatalf("SetStart(%q): %v", name, err)
	}
}

func newEngine(t *testing.T, g *Graph, cfg ...EngineConfig) *Engine {
	t.Helper()
	e, err := NewEngine(g, cfg...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// --- State tests ---

func TestStateCloneIndependence(t *testing.T) {
	s := NewState()
	s.Data["key"] = "original"
	s.Metadata["m"] = "meta"

	c := s.Clone()
	c.Data["key"] = "modified"
	c.Metadata["m"] = "changed"

	if s.Data["key"] != "original" {
		t.Errorf("Clone modified original Data")
	}
	if s.Metadata["m"] != "meta" {
		t.Errorf("Clone modified original Metadata")
	}
}

func TestStateGet(t *testing.T) {
	s := NewState()
	s.Data["count"] = 42

	v, ok := Get[int](s, "count")
	if !ok || v != 42 {
		t.Errorf("Get[int] = %v, %v; want 42, true", v, ok)
	}

	// Wrong type
	_, ok = Get[string](s, "count")
	if ok {
		t.Error("Get[string] should return false for int value")
	}

	// Missing key
	_, ok = Get[int](s, "missing")
	if ok {
		t.Error("Get should return false for missing key")
	}
}

func TestStateSet(t *testing.T) {
	s := NewState()
	s2 := Set(s, "x", 99)

	if _, ok := s.Data["x"]; ok {
		t.Error("Set should not mutate original state")
	}
	if v, _ := Get[int](s2, "x"); v != 99 {
		t.Errorf("Set: want 99, got %v", v)
	}
}

// --- Graph validation tests ---

func TestValidateNoStartNode(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	if err := g.Validate(); err == nil {
		t.Error("expected error for missing start node")
	}
}

func TestValidateNoNodes(t *testing.T) {
	g := NewGraph()
	// Can't even set start since there are no nodes, so validate directly
	if err := g.Validate(); err == nil {
		t.Error("expected error for empty graph")
	}
}

func TestValidateNodeNoEdges(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddNode(t, g, "b", noop)
	mustAddEdge(t, g, "a", "b")
	mustSetStart(t, g, "a")
	// "b" has no edges
	if err := g.Validate(); err == nil {
		t.Error("expected error for node with no edges")
	}
}

func TestValidateUnknownEdgeTarget(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustSetStart(t, g, "a")
	// manually add a bad edge to bypass AddEdge validation
	g.nodes["a"].edges = append(g.nodes["a"].edges, edge{to: "nonexistent"})
	if err := g.Validate(); err == nil {
		t.Error("expected error for edge to unknown node")
	}
}

func TestAddNodeErrors(t *testing.T) {
	g := NewGraph()

	if err := g.AddNode("", noop); err == nil {
		t.Error("expected error for empty node name")
	}
	if err := g.AddNode(EndNode, noop); err == nil {
		t.Error("expected error for reserved node name")
	}
	if err := g.AddNode("a", nil); err == nil {
		t.Error("expected error for nil handler")
	}

	mustAddNode(t, g, "a", noop)
	if err := g.AddNode("a", noop); err == nil {
		t.Error("expected error for duplicate node")
	}
}

func TestAddEdgeErrors(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)

	if err := g.AddEdge("unknown", "a"); err == nil {
		t.Error("expected error for unknown source")
	}
	if err := g.AddEdge("a", "unknown"); err == nil {
		t.Error("expected error for unknown target")
	}
}

func TestAddConditionalEdgeErrors(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)

	if err := g.AddConditionalEdge("unknown", func(State) string { return EndNode }); err == nil {
		t.Error("expected error for unknown source")
	}
	if err := g.AddConditionalEdge("a", nil); err == nil {
		t.Error("expected error for nil condition")
	}
}

func TestSetStartErrors(t *testing.T) {
	g := NewGraph()
	if err := g.SetStart("nonexistent"); err == nil {
		t.Error("expected error for unknown start node")
	}
}

// --- Linear graph test ---

func TestLinearGraph(t *testing.T) {
	g := NewGraph()

	mustAddNode(t, g, "a", func(_ context.Context, s State) (State, error) {
		return Set(s, "visited_a", true), nil
	})
	mustAddNode(t, g, "b", func(_ context.Context, s State) (State, error) {
		return Set(s, "visited_b", true), nil
	})
	mustAddNode(t, g, "c", func(_ context.Context, s State) (State, error) {
		return Set(s, "visited_c", true), nil
	})
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "run1", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d; want 3", result.Steps)
	}
	for _, key := range []string{"visited_a", "visited_b", "visited_c"} {
		v, ok := Get[bool](result.FinalState, key)
		if !ok || !v {
			t.Errorf("expected %s = true in final state", key)
		}
	}
}

// --- Conditional branching ---

func TestConditionalBranching(t *testing.T) {
	for _, tc := range []struct {
		flag   bool
		expect string
	}{
		{true, "b"},
		{false, "c"},
	} {
		g := NewGraph()
		mustAddNode(t, g, "a", noop)
		mustAddNode(t, g, "b", func(_ context.Context, s State) (State, error) {
			return Set(s, "branch", "b"), nil
		})
		mustAddNode(t, g, "c", func(_ context.Context, s State) (State, error) {
			return Set(s, "branch", "c"), nil
		})

		if err := g.AddConditionalEdge("a", func(s State) string {
			v, _ := Get[bool](s, "flag")
			if v {
				return "b"
			}
			return "c"
		}); err != nil {
			t.Fatalf("AddConditionalEdge: %v", err)
		}
		mustAddEdge(t, g, "b", EndNode)
		mustAddEdge(t, g, "c", EndNode)
		mustSetStart(t, g, "a")

		e := newEngine(t, g)
		initial := Set(NewState(), "flag", tc.flag)
		result, err := e.Run(context.Background(), "run-branch", initial)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if result.Status != RunStatusCompleted {
			t.Errorf("flag=%v: status = %v", tc.flag, result.Status)
		}
		branch, _ := Get[string](result.FinalState, "branch")
		if branch != tc.expect {
			t.Errorf("flag=%v: branch = %q; want %q", tc.flag, branch, tc.expect)
		}
	}
}

// --- Cycle with termination ---

func TestCycleWithTermination(t *testing.T) {
	g := NewGraph()

	mustAddNode(t, g, "a", func(_ context.Context, s State) (State, error) {
		count, _ := Get[int](s, "count")
		return Set(s, "count", count+1), nil
	})
	mustAddNode(t, g, "b", noop)

	if err := g.AddConditionalEdge("a", func(s State) string {
		count, _ := Get[int](s, "count")
		if count < 3 {
			return "b"
		}
		return EndNode
	}); err != nil {
		t.Fatalf("AddConditionalEdge: %v", err)
	}
	mustAddEdge(t, g, "b", "a")
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "run-cycle", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	count, _ := Get[int](result.FinalState, "count")
	if count != 3 {
		t.Errorf("count = %d; want 3", count)
	}
}

// --- MaxSteps enforcement ---

func TestMaxStepsEnforcement(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", "a") // infinite self-loop
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{MaxSteps: 5, DefaultNodeTimeout: time.Second})
	result, err := e.Run(context.Background(), "run-maxsteps", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
	if result.Steps != 5 {
		t.Errorf("Steps = %d; want 5", result.Steps)
	}
	if result.Error == "" {
		t.Error("expected error message for max steps exceeded")
	}
}

// --- Checkpoint and resume ---

func TestCheckpointAndResume(t *testing.T) {
	store := NewMemoryCheckpointStore()

	// Build a graph with 3 nodes. We'll cancel after first node runs.
	nodeOrder := make([]string, 0)
	var mu sync.Mutex
	recordNode := func(name string) NodeFunc {
		return func(_ context.Context, s State) (State, error) {
			mu.Lock()
			nodeOrder = append(nodeOrder, name)
			mu.Unlock()
			return Set(s, "last", name), nil
		}
	}

	g := NewGraph()
	mustAddNode(t, g, "n1", recordNode("n1"))
	mustAddNode(t, g, "n2", recordNode("n2"))
	mustAddNode(t, g, "n3", recordNode("n3"))
	mustAddEdge(t, g, "n1", "n2")
	mustAddEdge(t, g, "n2", "n3")
	mustAddEdge(t, g, "n3", EndNode)
	mustSetStart(t, g, "n1")

	cfg := EngineConfig{
		MaxSteps:           100,
		DefaultNodeTimeout: time.Second,
		CheckpointStore:    store,
	}

	e := newEngine(t, g, cfg)

	// Use a context that cancels after the first checkpoint is saved.
	// We hook into OnCheckpoint to cancel after the first save.
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Hooks = Hooks{
		OnCheckpoint: func(cp Checkpoint) {
			if cp.Step == 1 {
				cancel() // cancel after n1 completes
			}
		},
	}
	// Re-create engine with hooks
	e2, err := NewEngine(g, cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := e2.Run(ctx, "run-resume", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Should be stopped (context cancelled)
	if result.Status != RunStatusStopped {
		t.Errorf("expected stopped status, got %v", result.Status)
	}
	_ = e

	// Verify checkpoint exists
	ids, _ := store.List(context.Background())
	if len(ids) == 0 {
		t.Fatal("expected checkpoint to be saved")
	}

	// Resume
	ctx2 := context.Background()
	result2, err := e2.Resume(ctx2, "run-resume")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result2.Status != RunStatusCompleted {
		t.Errorf("Resume status = %v; want completed", result2.Status)
	}

	// All nodes should have been visited
	mu.Lock()
	defer mu.Unlock()
	if len(nodeOrder) < 3 {
		t.Errorf("expected all 3 nodes visited, got %v", nodeOrder)
	}
}

func TestResumeNoCheckpointStore(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g) // no checkpoint store
	_, err := e.Resume(context.Background(), "run-x")
	if err == nil {
		t.Error("expected error when no checkpoint store")
	}
}

func TestResumeNoCheckpoint(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:        100,
		CheckpointStore: NewMemoryCheckpointStore(),
	})
	_, err := e.Resume(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing checkpoint")
	}
}

// --- Node error handling ---

func TestNodeError(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", func(_ context.Context, s State) (State, error) {
		return s, errors.New("node failed")
	})
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "run-err", NewState())
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

// --- Node timeout ---

func TestNodeTimeout(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "slow", func(ctx context.Context, s State) (State, error) {
		select {
		case <-time.After(5 * time.Second):
			return s, nil
		case <-ctx.Done():
			return s, ctx.Err()
		}
	}, WithNodeTimeout(50*time.Millisecond))
	mustAddEdge(t, g, "slow", EndNode)
	mustSetStart(t, g, "slow")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "run-timeout", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
}

// --- Hooks ---

func TestHooks(t *testing.T) {
	var (
		nodeStarts    []string
		nodeEnds      []string
		nodeErrors    []string
		checkpoints   []Checkpoint
		cyclesAt      []int
		mu            sync.Mutex
	)

	hooks := Hooks{
		OnNodeStart: func(name string, _ State) {
			mu.Lock(); nodeStarts = append(nodeStarts, name); mu.Unlock()
		},
		OnNodeEnd: func(name string, _ State) {
			mu.Lock(); nodeEnds = append(nodeEnds, name); mu.Unlock()
		},
		OnNodeError: func(name string, _ error) {
			mu.Lock(); nodeErrors = append(nodeErrors, name); mu.Unlock()
		},
		OnCheckpoint: func(cp Checkpoint) {
			mu.Lock(); checkpoints = append(checkpoints, cp); mu.Unlock()
		},
		OnCycleDetected: func(_ string, step int) {
			mu.Lock(); cyclesAt = append(cyclesAt, step); mu.Unlock()
		},
	}

	store := NewMemoryCheckpointStore()

	g := NewGraph()
	counter := 0
	mustAddNode(t, g, "loop", func(_ context.Context, s State) (State, error) {
		counter++
		return Set(s, "counter", counter), nil
	})
	if err := g.AddConditionalEdge("loop", func(s State) string {
		v, _ := Get[int](s, "counter")
		if v < 2 {
			return "loop" // self-loop — triggers cycle hook
		}
		return EndNode
	}); err != nil {
		t.Fatal(err)
	}
	mustSetStart(t, g, "loop")

	e, err := NewEngine(g, EngineConfig{
		MaxSteps:           100,
		DefaultNodeTimeout: time.Second,
		CheckpointStore:    store,
		Hooks:              hooks,
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	_, err = e.Run(context.Background(), "run-hooks", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(nodeStarts) != 2 {
		t.Errorf("OnNodeStart called %d times; want 2", len(nodeStarts))
	}
	if len(nodeEnds) != 2 {
		t.Errorf("OnNodeEnd called %d times; want 2", len(nodeEnds))
	}
	if len(checkpoints) != 2 {
		t.Errorf("OnCheckpoint called %d times; want 2", len(checkpoints))
	}
	if len(cyclesAt) != 1 {
		t.Errorf("OnCycleDetected called %d times; want 1", len(cyclesAt))
	}
	if len(nodeErrors) != 0 {
		t.Errorf("OnNodeError called unexpectedly: %v", nodeErrors)
	}
}

func TestHookNodeError(t *testing.T) {
	var errored []string

	g := NewGraph()
	mustAddNode(t, g, "bad", func(_ context.Context, s State) (State, error) {
		return s, fmt.Errorf("boom")
	})
	mustAddEdge(t, g, "bad", EndNode)
	mustSetStart(t, g, "bad")

	e, err := NewEngine(g, EngineConfig{
		MaxSteps:           10,
		DefaultNodeTimeout: time.Second,
		Hooks: Hooks{
			OnNodeError: func(name string, _ error) {
				errored = append(errored, name)
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, _ := e.Run(context.Background(), "r", NewState())
	if result.Status != RunStatusFailed {
		t.Errorf("want failed, got %v", result.Status)
	}
	if len(errored) != 1 || errored[0] != "bad" {
		t.Errorf("OnNodeError not called correctly: %v", errored)
	}
}

// --- MemoryCheckpointStore ---

func TestMemoryCheckpointStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	cp := Checkpoint{
		RunID:       "r1",
		CurrentNode: "a",
		Step:        3,
		State:       NewState(),
		SavedAt:     time.Now(),
	}

	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := store.Load(ctx, "r1")
	if err != nil || !found {
		t.Fatalf("Load: found=%v, err=%v", found, err)
	}
	if loaded.Step != 3 || loaded.CurrentNode != "a" {
		t.Errorf("Load: unexpected checkpoint %+v", loaded)
	}

	ids, err := store.List(ctx)
	if err != nil || len(ids) != 1 || ids[0] != "r1" {
		t.Errorf("List: ids=%v err=%v", ids, err)
	}

	if err := store.Delete(ctx, "r1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, _ = store.Load(ctx, "r1")
	if found {
		t.Error("expected checkpoint to be deleted")
	}
}

// --- Context cancellation ---

func TestContextCancellation(t *testing.T) {
	g := NewGraph()
	// Use a node that blocks until context is done, ensuring cancellation is noticed.
	mustAddNode(t, g, "a", func(ctx context.Context, s State) (State, error) {
		// Yield briefly so context cancellation can propagate between steps.
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-time.After(time.Millisecond):
			return s, nil
		}
	})
	mustAddEdge(t, g, "a", "a") // infinite loop
	mustSetStart(t, g, "a")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	e := newEngine(t, g, EngineConfig{MaxSteps: 10000, DefaultNodeTimeout: time.Second})
	result, err := e.Run(ctx, "run-cancel", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The node may return an error (context deadline exceeded) causing RunStatusFailed,
	// or the outer loop may catch ctx.Err() causing RunStatusStopped.
	// Both are valid — the key thing is the run terminates without hitting MaxSteps.
	if result.Status == RunStatusCompleted {
		t.Errorf("Status should not be completed for cancelled run")
	}
	if result.Steps >= 10000 {
		t.Errorf("expected cancellation before MaxSteps, got %d steps", result.Steps)
	}
}
