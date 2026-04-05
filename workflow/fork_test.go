package workflow

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAddForkNode_Valid verifies that a fork node with valid branches is added
// to the graph without error and appears as a single node.
func TestAddForkNode_Valid(t *testing.T) {
	g := NewGraph()
	branches := map[string]NodeFunc{
		"branch_a": func(_ context.Context, s State) (State, error) { return s, nil },
		"branch_b": func(_ context.Context, s State) (State, error) { return s, nil },
	}
	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	if _, ok := g.nodes["fork"]; !ok {
		t.Error("fork node not present in graph after AddForkNode")
	}
}

// TestAddForkNode_EmptyName verifies that an empty node name is rejected.
func TestAddForkNode_EmptyName(t *testing.T) {
	g := NewGraph()
	branches := map[string]NodeFunc{
		"b": noop,
	}
	if err := g.AddForkNode("", branches, MergeAll); err == nil {
		t.Error("expected error for empty fork node name")
	}
}

// TestAddForkNode_NoBranches verifies that an empty branches map is rejected.
func TestAddForkNode_NoBranches(t *testing.T) {
	g := NewGraph()
	if err := g.AddForkNode("fork", map[string]NodeFunc{}, MergeAll); err == nil {
		t.Error("expected error for empty branches map")
	}
}

// TestAddForkNode_NilMerge verifies that a nil merge function is rejected.
func TestAddForkNode_NilMerge(t *testing.T) {
	g := NewGraph()
	branches := map[string]NodeFunc{
		"b": noop,
	}
	if err := g.AddForkNode("fork", branches, nil); err == nil {
		t.Error("expected error for nil merge function")
	}
}

// TestAddForkNode_NilBranch verifies that a nil branch function is rejected.
func TestAddForkNode_NilBranch(t *testing.T) {
	g := NewGraph()
	branches := map[string]NodeFunc{
		"a": noop,
		"b": nil,
	}
	if err := g.AddForkNode("fork", branches, MergeAll); err == nil {
		t.Error("expected error for nil branch function")
	}
}

// TestAddForkNode_Duplicate verifies that adding a fork node with the same
// name as an existing node returns an error.
func TestAddForkNode_Duplicate(t *testing.T) {
	g := NewGraph()
	mustAddNode(t, g, "fork", noop)

	branches := map[string]NodeFunc{"b": noop}
	if err := g.AddForkNode("fork", branches, MergeAll); err == nil {
		t.Error("expected error for duplicate node name")
	}
}

// TestForkNode_ParallelExecution verifies that branches execute concurrently.
// Each branch blocks on a channel; we verify all branches are "in flight" at
// the same time before releasing them.
func TestForkNode_ParallelExecution(t *testing.T) {
	const numBranches = 3

	// ready is incremented by each branch when it starts; release is closed to
	// let all branches proceed.
	var ready int32
	release := make(chan struct{})

	branches := make(map[string]NodeFunc, numBranches)
	for i := range numBranches {
		name := fmt.Sprintf("b%d", i)
		branches[name] = func(_ context.Context, s State) (State, error) {
			atomic.AddInt32(&ready, 1)
			<-release
			return s, nil
		}
	}

	g := NewGraph()
	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g)

	// Run in background; once all branches are ready, release them.
	done := make(chan *RunResult)
	go func() {
		result, err := e.Run(context.Background(), "parallel", NewState())
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		done <- result
	}()

	// Wait until all branches have started (with a timeout to avoid hanging).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&ready) == int32(numBranches) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if atomic.LoadInt32(&ready) != int32(numBranches) {
		close(release)
		t.Fatalf("branches did not all start; only %d of %d started (parallelism not working)", atomic.LoadInt32(&ready), numBranches)
	}

	// All branches are running concurrently — release them.
	close(release)

	select {
	case result := <-done:
		if result.Status != RunStatusCompleted {
			t.Errorf("Status = %v; want completed", result.Status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for fork node to complete")
	}
}

// TestForkNode_MergeAll verifies that MergeAll combines Data from all branches.
func TestForkNode_MergeAll(t *testing.T) {
	base := NewState()
	base.Data["base_key"] = "base_val"

	branches := map[string]State{
		"a": {Data: map[string]any{"a_key": "a_val"}, Metadata: map[string]string{"am": "av"}},
		"b": {Data: map[string]any{"b_key": "b_val"}, Metadata: map[string]string{"bm": "bv"}},
	}

	result, err := MergeAll(base, branches)
	if err != nil {
		t.Fatalf("MergeAll: %v", err)
	}

	// Base key preserved.
	if v, ok := result.Data["base_key"]; !ok || v != "base_val" {
		t.Errorf("base_key = %v; want base_val", v)
	}
	// Branch data merged.
	if v, ok := result.Data["a_key"]; !ok || v != "a_val" {
		t.Errorf("a_key = %v; want a_val", v)
	}
	if v, ok := result.Data["b_key"]; !ok || v != "b_val" {
		t.Errorf("b_key = %v; want b_val", v)
	}
	// Branch metadata merged.
	if v, ok := result.Metadata["am"]; !ok || v != "av" {
		t.Errorf("metadata am = %v; want av", v)
	}
	if v, ok := result.Metadata["bm"]; !ok || v != "bv" {
		t.Errorf("metadata bm = %v; want bv", v)
	}
}

// TestForkNode_MergeKeyed verifies that MergeKeyed stores each branch's Data
// under its branch name in the result.
func TestForkNode_MergeKeyed(t *testing.T) {
	base := NewState()

	branches := map[string]State{
		"alpha": {Data: map[string]any{"x": 1}, Metadata: map[string]string{}},
		"beta":  {Data: map[string]any{"y": 2}, Metadata: map[string]string{}},
	}

	result, err := MergeKeyed(base, branches)
	if err != nil {
		t.Fatalf("MergeKeyed: %v", err)
	}

	alphaRaw, ok := result.Data["alpha"]
	if !ok {
		t.Fatal("expected 'alpha' key in result Data")
	}
	alphaMap, ok := alphaRaw.(map[string]any)
	if !ok {
		t.Fatalf("alpha value is %T; want map[string]any", alphaRaw)
	}
	if alphaMap["x"] != 1 {
		t.Errorf("alpha.x = %v; want 1", alphaMap["x"])
	}

	betaRaw, ok := result.Data["beta"]
	if !ok {
		t.Fatal("expected 'beta' key in result Data")
	}
	betaMap, ok := betaRaw.(map[string]any)
	if !ok {
		t.Fatalf("beta value is %T; want map[string]any", betaRaw)
	}
	if betaMap["y"] != 2 {
		t.Errorf("beta.y = %v; want 2", betaMap["y"])
	}
}

// TestForkNode_BranchError verifies that an error in any branch causes the
// fork node to return an error, with the branch name embedded.
func TestForkNode_BranchError(t *testing.T) {
	g := NewGraph()

	branches := map[string]NodeFunc{
		"good": func(_ context.Context, s State) (State, error) { return s, nil },
		"bad":  func(_ context.Context, s State) (State, error) { return s, fmt.Errorf("branch exploded") },
	}

	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "branch-err", NewState())
	if err != nil {
		t.Fatalf("Run returned unexpected Go error: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error in result")
	}
}

// TestForkNode_ContextCancellation verifies that branches respect context
// cancellation. Each branch should exit when the context is cancelled.
func TestForkNode_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	branches := map[string]NodeFunc{
		"slow_a": func(ctx context.Context, s State) (State, error) {
			select {
			case <-ctx.Done():
				return s, ctx.Err()
			case <-time.After(5 * time.Second):
				return s, nil
			}
		},
		"slow_b": func(ctx context.Context, s State) (State, error) {
			select {
			case <-ctx.Done():
				return s, ctx.Err()
			case <-time.After(5 * time.Second):
				return s, nil
			}
		},
	}

	g := NewGraph()
	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g, EngineConfig{MaxSteps: 10, DefaultNodeTimeout: 10 * time.Second})

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := e.Run(ctx, "ctx-cancel", NewState())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run returned unexpected Go error: %v", err)
	}
	if result.Status == RunStatusCompleted {
		t.Error("run should not complete when context is cancelled")
	}
	// Should finish well before the 5 second branch timeout.
	if elapsed > 2*time.Second {
		t.Errorf("took %v; expected cancellation much sooner", elapsed)
	}
}

// TestForkNode_InGraph builds a complete graph with a fork node and runs it
// through the Engine, verifying that the merged output contains data from
// all branches.
//
// Graph: start → fork(branch_a, branch_b) → collect → END
func TestForkNode_InGraph(t *testing.T) {
	var collectSaw map[string]any

	g := NewGraph()

	branches := map[string]NodeFunc{
		"branch_a": func(_ context.Context, s State) (State, error) {
			return Set(s, "a", "from_a"), nil
		},
		"branch_b": func(_ context.Context, s State) (State, error) {
			return Set(s, "b", "from_b"), nil
		},
	}

	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}

	mustAddNode(t, g, "start", noop)
	mustAddNode(t, g, "collect", func(_ context.Context, s State) (State, error) {
		collectSaw = make(map[string]any, len(s.Data))
		maps.Copy(collectSaw, s.Data)
		return s, nil
	})

	mustAddEdge(t, g, "start", "fork")
	mustAddEdge(t, g, "fork", "collect")
	mustAddEdge(t, g, "collect", EndNode)
	mustSetStart(t, g, "start")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "in-graph", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v (%s); want completed", result.Status, result.Error)
	}

	// collect node should have seen both branch outputs.
	if collectSaw == nil {
		t.Fatal("collect node did not run")
	}
	if v, ok := collectSaw["a"]; !ok || v != "from_a" {
		t.Errorf("a = %v; want from_a", v)
	}
	if v, ok := collectSaw["b"]; !ok || v != "from_b" {
		t.Errorf("b = %v; want from_b", v)
	}

	// Final state must also contain both.
	if v, ok := Get[string](result.FinalState, "a"); !ok || v != "from_a" {
		t.Errorf("FinalState a = %v; want from_a", v)
	}
	if v, ok := Get[string](result.FinalState, "b"); !ok || v != "from_b" {
		t.Errorf("FinalState b = %v; want from_b", v)
	}
}

// TestForkNode_WithTimeout verifies that a per-node timeout (via NodeOption)
// is respected; a fork node whose branches take too long should fail with the
// node timeout.
func TestForkNode_WithTimeout(t *testing.T) {
	branches := map[string]NodeFunc{
		"slow": func(ctx context.Context, s State) (State, error) {
			select {
			case <-ctx.Done():
				return s, ctx.Err()
			case <-time.After(5 * time.Second):
				return s, nil
			}
		},
	}

	g := NewGraph()
	// 50 ms per-node timeout — much shorter than the 5 s branch sleep.
	if err := g.AddForkNode("fork", branches, MergeAll, WithNodeTimeout(50*time.Millisecond)); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g)

	start := time.Now()
	result, err := e.Run(context.Background(), "fork-timeout", NewState())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run returned unexpected Go error: %v", err)
	}
	if result.Status != RunStatusFailed {
		t.Errorf("Status = %v; want failed (timeout)", result.Status)
	}
	// Should fail well under a second.
	if elapsed > 2*time.Second {
		t.Errorf("took %v; expected failure within timeout window", elapsed)
	}
}

// TestAddForkNode_ReservedName verifies that EndNode is rejected as a fork
// node name.
func TestAddForkNode_ReservedName(t *testing.T) {
	g := NewGraph()
	branches := map[string]NodeFunc{"b": noop}
	if err := g.AddForkNode(EndNode, branches, MergeAll); err == nil {
		t.Error("expected error for reserved EndNode name")
	}
}

// TestForkNode_MergeAll_NoMutateBase verifies that MergeAll does not mutate
// the original base state.
func TestForkNode_MergeAll_NoMutateBase(t *testing.T) {
	base := NewState()
	base.Data["orig"] = "original"

	branches := map[string]State{
		"a": {Data: map[string]any{"new_key": "new_val"}, Metadata: map[string]string{}},
	}

	_, err := MergeAll(base, branches)
	if err != nil {
		t.Fatalf("MergeAll: %v", err)
	}

	if _, ok := base.Data["new_key"]; ok {
		t.Error("MergeAll mutated base state Data")
	}
	if base.Data["orig"] != "original" {
		t.Error("MergeAll corrupted base state")
	}
}

// TestForkNode_SingleBranch verifies that a fork node with exactly one branch
// works correctly end-to-end.
func TestForkNode_SingleBranch(t *testing.T) {
	g := NewGraph()

	branches := map[string]NodeFunc{
		"only": func(_ context.Context, s State) (State, error) {
			return Set(s, "done", true), nil
		},
	}

	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g)
	result, err := e.Run(context.Background(), "single-branch", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	v, ok := Get[bool](result.FinalState, "done")
	if !ok || !v {
		t.Error("expected done=true in final state")
	}
}

// TestForkNode_BranchNodeName verifies that each branch receives a NodeName
// composed of "forkName/branchName".
func TestForkNode_BranchNodeName(t *testing.T) {
	var mu sync.Mutex
	seenNames := make(map[string]bool)

	branches := map[string]NodeFunc{
		"a": func(_ context.Context, s State) (State, error) {
			mu.Lock()
			seenNames[s.NodeName] = true
			mu.Unlock()
			return s, nil
		},
		"b": func(_ context.Context, s State) (State, error) {
			mu.Lock()
			seenNames[s.NodeName] = true
			mu.Unlock()
			return s, nil
		},
	}

	g := NewGraph()
	if err := g.AddForkNode("fork", branches, MergeAll); err != nil {
		t.Fatalf("AddForkNode: %v", err)
	}
	mustAddEdge(t, g, "fork", EndNode)
	mustSetStart(t, g, "fork")

	e := newEngine(t, g)
	_, err := e.Run(context.Background(), "node-name", NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, expect := range []string{"fork/a", "fork/b"} {
		if !seenNames[expect] {
			t.Errorf("expected branch NodeName %q; got names: %v", expect, seenNames)
		}
	}
}
