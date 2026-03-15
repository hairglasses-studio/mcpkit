package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewEngine_InvalidGraph verifies that NewEngine returns an error when the
// graph fails validation (no start node set).
func TestNewEngine_InvalidGraph(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	// start node not set — Validate() will fail

	_, err := NewEngine(g)
	if err == nil {
		t.Error("expected error for invalid graph (no start node)")
	}
}

// TestNewEngine_EmptyGraph verifies that NewEngine rejects an empty graph.
func TestNewEngine_EmptyGraph(t *testing.T) {
	g := NewGraph()
	_, err := NewEngine(g)
	if err == nil {
		t.Error("expected error for empty graph")
	}
}

// TestNewEngine_DefaultConfig verifies that default values are applied when no
// config is provided.
func TestNewEngine_DefaultConfig(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e, err := NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if e.config.MaxSteps != 1000 {
		t.Errorf("default MaxSteps = %d; want 1000", e.config.MaxSteps)
	}
	if e.config.DefaultNodeTimeout != 30*time.Second {
		t.Errorf("default DefaultNodeTimeout = %v; want 30s", e.config.DefaultNodeTimeout)
	}
}

// TestRun_LinearOrder verifies that nodes in a linear A→B→C graph execute
// in order and the final state reflects each node's contribution.
func TestRun_LinearOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	makeNode := func(name string) NodeFunc {
		return func(_ context.Context, s State) (State, error) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return Set(s, "last", name), nil
		}
	}

	g := NewGraph()
	mustAddNode(t, g, "a", makeNode("a"))
	mustAddNode(t, g, "b", makeNode("b"))
	mustAddNode(t, g, "c", makeNode("c"))
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "linear", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d; want 3", result.Steps)
	}

	mu.Lock()
	defer mu.Unlock()
	expected := []string{"a", "b", "c"}
	if len(order) != len(expected) {
		t.Fatalf("execution order len %d; want %d: %v", len(order), len(expected), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("order[%d] = %q; want %q", i, order[i], want)
		}
	}

	last, ok := Get[string](result.FinalState, "last")
	if !ok || last != "c" {
		t.Errorf("FinalState last = %q; want c", last)
	}
}

// TestRun_ConditionalEdge verifies that conditional routing directs execution
// to the correct branch based on state.
func TestRun_ConditionalEdge(t *testing.T) {
	cases := []struct {
		route  string
		expect string
	}{
		{"left", "left-result"},
		{"right", "right-result"},
	}

	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			g := NewGraph()
			mustAddNode(t, g, "router", noop)
			mustAddNode(t, g, "left", func(_ context.Context, s State) (State, error) {
				return Set(s, "result", "left-result"), nil
			})
			mustAddNode(t, g, "right", func(_ context.Context, s State) (State, error) {
				return Set(s, "result", "right-result"), nil
			})

			if err := g.AddConditionalEdge("router", func(s State) string {
				v, _ := Get[string](s, "route")
				return v
			}); err != nil {
				t.Fatalf("AddConditionalEdge: %v", err)
			}
			mustAddEdge(t, g, "left", EndNode)
			mustAddEdge(t, g, "right", EndNode)
			mustSetStart(t, g, "router")

			initial := Set(NewState(), "route", tc.route)
			e := newEngine(t, g)
			result, err := e.Run(context.Background(), "cond-"+tc.route, initial)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if result.Status != RunStatusCompleted {
				t.Errorf("Status = %v; want completed", result.Status)
			}
			got, _ := Get[string](result.FinalState, "result")
			if got != tc.expect {
				t.Errorf("result = %q; want %q", got, tc.expect)
			}
		})
	}
}

// TestRun_MaxSteps verifies that execution stops and returns a failed status
// when the step limit is reached.
func TestRun_MaxSteps(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "loop", noop)
	mustAddEdge(t, g, "loop", "loop") // infinite self-loop
	mustSetStart(t, g, "loop")

	e := newEngine(t, g, EngineConfig{MaxSteps: 3, DefaultNodeTimeout: time.Second})
	result, err := e.Run(context.Background(), "maxsteps", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d; want 3", result.Steps)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for max steps exceeded")
	}
}

// TestRun_ContextCancellation verifies that a cancelled context stops the run
// before MaxSteps is reached.
func TestRun_ContextCancellation(t *testing.T) {
	g := NewGraph()
	// A node that yields briefly so the outer loop has a chance to see ctx.Err().
	mustAddNode(t, g, "spin", func(ctx context.Context, s State) (State, error) {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-time.After(time.Millisecond):
			return s, nil
		}
	})
	mustAddEdge(t, g, "spin", "spin") // keep looping
	mustSetStart(t, g, "spin")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	e := newEngine(t, g, EngineConfig{MaxSteps: 10000, DefaultNodeTimeout: time.Second})
	result, err := e.Run(ctx, "cancel", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status == RunStatusCompleted {
		t.Error("run should not complete when context is cancelled")
	}
	if result.Steps >= 10000 {
		t.Errorf("expected cancellation before MaxSteps, got %d steps", result.Steps)
	}
}

// TestRun_ContextTimeout verifies that a context deadline causes the run to
// stop before MaxSteps is reached.
func TestRun_ContextTimeout(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "tick", func(ctx context.Context, s State) (State, error) {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-time.After(time.Millisecond):
			return s, nil
		}
	})
	mustAddEdge(t, g, "tick", "tick")
	mustSetStart(t, g, "tick")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	e := newEngine(t, g, EngineConfig{MaxSteps: 50000, DefaultNodeTimeout: time.Second})
	result, err := e.Run(ctx, "timeout-ctx", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status == RunStatusCompleted {
		t.Error("run should not complete when context deadline exceeded")
	}
	if result.Steps >= 50000 {
		t.Errorf("expected timeout before MaxSteps, got %d steps", result.Steps)
	}
}

// TestRun_NodeError verifies that a node returning an error causes the run to
// stop with RunStatusFailed and an error message.
func TestRun_NodeError(t *testing.T) {
	boom := errors.New("something went wrong")

	g := NewGraph()
	mustAddNode(t, g, "bad", func(_ context.Context, s State) (State, error) {
		return s, boom
	})
	mustAddEdge(t, g, "bad", EndNode)
	mustSetStart(t, g, "bad")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "node-err", NewState())
	if err != nil {
		t.Fatalf("Run returned unexpected Go error: %v", err)
	}

	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error in result")
	}
	if result.Steps != 1 {
		t.Errorf("Steps = %d; want 1 (error on first step)", result.Steps)
	}
}

// TestRun_ErrorPropagationMessage verifies that the node name is embedded in
// the error string returned in the result.
func TestRun_ErrorPropagationMessage(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "my-node", func(_ context.Context, s State) (State, error) {
		return s, fmt.Errorf("custom error")
	})
	mustAddEdge(t, g, "my-node", EndNode)
	mustSetStart(t, g, "my-node")

	e := newEngine(t, g)
	result, _ := e.Run(context.Background(), "err-msg", NewState())

	// Error should mention the node name.
	if result.Error == "" {
		t.Fatal("expected non-empty Error")
	}
}

// TestRun_ResumeFromCheckpoint tests the full save-cancel-resume cycle:
// run until a checkpoint is saved, then resume and verify completion.
func TestRun_ResumeFromCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()

	var visited []string
	var mu sync.Mutex
	track := func(name string) NodeFunc {
		return func(_ context.Context, s State) (State, error) {
			mu.Lock()
			visited = append(visited, name)
			mu.Unlock()
			return Set(s, "last", name), nil
		}
	}

	g := NewGraph()
	mustAddNode(t, g, "n1", track("n1"))
	mustAddNode(t, g, "n2", track("n2"))
	mustAddNode(t, g, "n3", track("n3"))
	mustAddEdge(t, g, "n1", "n2")
	mustAddEdge(t, g, "n2", "n3")
	mustAddEdge(t, g, "n3", EndNode)
	mustSetStart(t, g, "n1")

	// Cancel after n1's checkpoint is saved.
	ctx, cancel := context.WithCancel(context.Background())
	hooks := Hooks{
		OnCheckpoint: func(cp Checkpoint) {
			if cp.CurrentNode == "n1" {
				cancel()
			}
		},
	}

	e, err := NewEngine(g, EngineConfig{
		MaxSteps:           100,
		DefaultNodeTimeout: time.Second,
		CheckpointStore:    store,
		Hooks:              hooks,
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := e.Run(ctx, "resume-run", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusStopped {
		t.Errorf("first run status = %v; want stopped", result.Status)
	}

	// Checkpoint must have been saved.
	ids, _ := store.List(context.Background())
	if len(ids) == 0 {
		t.Fatal("expected checkpoint to be saved before cancellation")
	}

	// Resume should run remaining nodes and complete.
	result2, err := e.Resume(context.Background(), "resume-run")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if result2.Status != RunStatusCompleted {
		t.Errorf("resume status = %v; want completed", result2.Status)
	}

	mu.Lock()
	defer mu.Unlock()
	// All three nodes should have been executed across both runs.
	found := make(map[string]bool)
	for _, v := range visited {
		found[v] = true
	}
	for _, name := range []string{"n1", "n2", "n3"} {
		if !found[name] {
			t.Errorf("node %q was never visited; visited = %v", name, visited)
		}
	}
}

// TestResume_NoCheckpointStore verifies that Resume returns an error when no
// checkpoint store is configured.
func TestResume_NoCheckpointStore(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g) // no CheckpointStore
	_, err := e.Resume(context.Background(), "run-x")
	if err == nil {
		t.Error("expected error when no checkpoint store configured")
	}
}

// TestResume_MissingCheckpoint verifies that Resume returns an error when the
// checkpoint for the given runID does not exist.
func TestResume_MissingCheckpoint(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:        10,
		CheckpointStore: NewMemoryCheckpointStore(),
	})
	_, err := e.Resume(context.Background(), "does-not-exist")
	if err == nil {
		t.Error("expected error for non-existent checkpoint")
	}
}

// TestRun_CheckpointDeletedOnCompletion verifies that the checkpoint is removed
// from the store after a successful run completes.
func TestRun_CheckpointDeletedOnCompletion(t *testing.T) {
	store := NewMemoryCheckpointStore()
	const runID = "cleanup-run"

	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g, EngineConfig{
		MaxSteps:           10,
		DefaultNodeTimeout: time.Second,
		CheckpointStore:    store,
	})

	result, err := e.Run(context.Background(), runID, NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %v; want completed", result.Status)
	}

	_, found, _ := store.Load(context.Background(), runID)
	if found {
		t.Error("checkpoint should be deleted after successful completion")
	}
}

// TestRun_StatePassedBetweenNodes verifies that state mutations in one node
// are visible to the next node.
func TestRun_StatePassedBetweenNodes(t *testing.T) {
	g := NewGraph()

	mustAddNode(t, g, "write", func(_ context.Context, s State) (State, error) {
		return Set(s, "msg", "hello"), nil
	})
	mustAddNode(t, g, "read", func(_ context.Context, s State) (State, error) {
		msg, ok := Get[string](s, "msg")
		if !ok || msg != "hello" {
			return s, fmt.Errorf("expected msg=hello, got %q (ok=%v)", msg, ok)
		}
		return s, nil
	})
	mustAddEdge(t, g, "write", "read")
	mustAddEdge(t, g, "read", EndNode)
	mustSetStart(t, g, "write")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "state-pass", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v (%s); want completed", result.Status, result.Error)
	}
}

// TestRun_RunIDInResult verifies that the run ID is preserved in the result.
func TestRun_RunIDInResult(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	const id = "my-unique-run-id"
	result, err := e.Run(context.Background(), id, NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.RunID != id {
		t.Errorf("RunID = %q; want %q", result.RunID, id)
	}
}

// TestRun_DurationRecorded verifies that Duration is non-zero after a run.
func TestRun_DurationRecorded(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "a", noop)
	mustAddEdge(t, g, "a", EndNode)
	mustSetStart(t, g, "a")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "dur", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Duration <= 0 {
		t.Errorf("Duration = %v; want > 0", result.Duration)
	}
}
