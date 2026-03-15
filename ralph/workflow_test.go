//go:build !official_sdk

package ralph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/workflow"
)

// buildSimpleGraph builds a single-node graph: start → __END__.
func buildSimpleGraph(t *testing.T) *workflow.Engine {
	t.Helper()
	g := workflow.NewGraph()
	if err := g.AddNode("start", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		s.Data["done"] = true
		return s, nil
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := g.AddEdge("start", workflow.EndNode); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := g.SetStart("start"); err != nil {
		t.Fatalf("SetStart: %v", err)
	}
	e, err := workflow.NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// buildFailingGraph builds a graph where the node always returns an error.
func buildFailingGraph(t *testing.T) *workflow.Engine {
	t.Helper()
	g := workflow.NewGraph()
	if err := g.AddNode("fail", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		return s, errors.New("intentional failure")
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := g.AddEdge("fail", workflow.EndNode); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := g.SetStart("fail"); err != nil {
		t.Fatalf("SetStart: %v", err)
	}
	e, err := workflow.NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// buildSlowGraph builds a graph where the node blocks until context is cancelled.
func buildSlowGraph(t *testing.T) *workflow.Engine {
	t.Helper()
	g := workflow.NewGraph()
	if err := g.AddNode("slow", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-time.After(10 * time.Second):
			return s, nil
		}
	}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := g.AddEdge("slow", workflow.EndNode); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := g.SetStart("slow"); err != nil {
		t.Fatalf("SetStart: %v", err)
	}
	// Use a short node timeout so the test doesn't hang if Stop doesn't work.
	e, err := workflow.NewEngine(g, workflow.EngineConfig{
		DefaultNodeTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

func TestNewWorkflowLoop_Valid(t *testing.T) {
	e := buildSimpleGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{Engine: e, RunID: "test-run"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if wl == nil {
		t.Fatal("expected non-nil WorkflowLoop")
	}
}

func TestNewWorkflowLoop_NilEngine(t *testing.T) {
	_, err := NewWorkflowLoop(WorkflowConfig{Engine: nil})
	if err == nil {
		t.Fatal("expected error for nil engine, got nil")
	}
}

func TestWorkflowLoop_Run_Success(t *testing.T) {
	e := buildSimpleGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "success-run",
		InitialState: workflow.NewState(),
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	if runErr := wl.Run(context.Background()); runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}

	p := wl.Status()
	if p.Status != StatusCompleted {
		t.Errorf("expected StatusCompleted, got %q", p.Status)
	}
}

func TestWorkflowLoop_Run_Failed(t *testing.T) {
	e := buildFailingGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "fail-run",
		InitialState: workflow.NewState(),
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	runErr := wl.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error from failed workflow, got nil")
	}

	p := wl.Status()
	if p.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %q", p.Status)
	}
}

func TestWorkflowLoop_Stop(t *testing.T) {
	e := buildSlowGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "stop-run",
		InitialState: workflow.NewState(),
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- wl.Run(context.Background())
	}()

	// Give the goroutine a moment to start before stopping.
	time.Sleep(20 * time.Millisecond)
	wl.Stop()

	select {
	case <-done:
		// Run returned — either stopped or failed due to context cancellation.
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not cause Run() to return within timeout")
	}
}

func TestWorkflowLoop_Hooks(t *testing.T) {
	e := buildSimpleGraph(t)

	var iterationStartCalled int
	var iterationEndCalled int
	var lastEntry IterationLog

	hooks := Hooks{
		OnIterationStart: func(iteration int) {
			iterationStartCalled++
		},
		OnIterationEnd: func(entry IterationLog) {
			iterationEndCalled++
			lastEntry = entry
		},
	}

	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "hooks-run",
		InitialState: workflow.NewState(),
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	if err := wl.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if iterationStartCalled != 1 {
		t.Errorf("expected OnIterationStart called once, got %d", iterationStartCalled)
	}
	if iterationEndCalled != 1 {
		t.Errorf("expected OnIterationEnd called once, got %d", iterationEndCalled)
	}
	if lastEntry.Iteration != 1 {
		t.Errorf("expected entry.Iteration=1, got %d", lastEntry.Iteration)
	}
}

func TestWorkflowLoop_MaxDuration(t *testing.T) {
	e := buildSlowGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "maxdur-run",
		InitialState: workflow.NewState(),
		MaxDuration:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	start := time.Now()
	// Run should return quickly due to MaxDuration.
	_ = wl.Run(context.Background())
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("MaxDuration did not limit execution: elapsed %v", elapsed)
	}
}

func TestWorkflowLoop_Status(t *testing.T) {
	e := buildSimpleGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "status-run",
		InitialState: workflow.NewState(),
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	// Before Run: progress should be zero value (empty status).
	before := wl.Status()
	if before.Status != "" {
		t.Errorf("expected empty status before Run, got %q", before.Status)
	}

	if err := wl.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	after := wl.Status()
	if after.Status != StatusCompleted {
		t.Errorf("expected StatusCompleted after Run, got %q", after.Status)
	}
	if after.Iteration == 0 {
		t.Error("expected Iteration > 0 after successful run")
	}
	if len(after.Log) == 0 {
		t.Error("expected at least one log entry after run")
	}
	if after.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
}
