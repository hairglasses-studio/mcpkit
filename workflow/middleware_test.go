package workflow

import (
	"context"
	"testing"
)

func TestWrapNodeFunc_Single(t *testing.T) {
	var log []string

	nodeFn := func(ctx context.Context, state State) (State, error) {
		log = append(log, "node")
		return Set(state, "result", "done"), nil
	}

	mw := func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			log = append(log, "before:"+nodeName)
			s, err := next(ctx, state)
			log = append(log, "after:"+nodeName)
			return s, err
		}
	}

	wrapped := WrapNodeFunc(nodeFn, "process", mw)
	state, err := wrapped(context.Background(), NewState())
	if err != nil {
		t.Fatal(err)
	}

	v, ok := Get[string](state, "result")
	if !ok || v != "done" {
		t.Fatalf("expected result 'done', got %q", v)
	}

	expected := []string{"before:process", "node", "after:process"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], e)
		}
	}
}

func TestWrapNodeFunc_MultipleMiddleware(t *testing.T) {
	var log []string

	nodeFn := func(ctx context.Context, state State) (State, error) {
		log = append(log, "node")
		return state, nil
	}

	makeMW := func(id string) NodeMiddleware {
		return func(nodeName string, next NodeFunc) NodeFunc {
			return func(ctx context.Context, state State) (State, error) {
				log = append(log, id+":before")
				s, err := next(ctx, state)
				log = append(log, id+":after")
				return s, err
			}
		}
	}

	wrapped := WrapNodeFunc(nodeFn, "n", makeMW("A"), makeMW("B"))
	_, err := wrapped(context.Background(), NewState())
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"A:before", "B:before", "node", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], e)
		}
	}
}

func TestEngine_NodeMiddleware(t *testing.T) {
	var visited []string

	g := NewGraph()
	_ = g.AddNode("start", func(ctx context.Context, state State) (State, error) {
		return Set(state, "step1", true), nil
	})
	_ = g.AddNode("end", func(ctx context.Context, state State) (State, error) {
		return Set(state, "step2", true), nil
	})
	_ = g.AddEdge("start", "end")
	_ = g.AddEdge("end", EndNode)
	_ = g.SetStart("start")

	mw := func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			visited = append(visited, nodeName)
			return next(ctx, state)
		}
	}

	engine, err := NewEngine(g, EngineConfig{
		NodeMiddleware: []NodeMiddleware{mw},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Run(context.Background(), "test-1", NewState())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %s: %s", result.Status, result.Error)
	}
	if len(visited) != 2 {
		t.Fatalf("expected 2 visited nodes, got %d: %v", len(visited), visited)
	}
	if visited[0] != "start" || visited[1] != "end" {
		t.Fatalf("unexpected visit order: %v", visited)
	}
}

func TestEngine_NodeMiddleware_CorrectName(t *testing.T) {
	var names []string

	g := NewGraph()
	_ = g.AddNode("alpha", func(ctx context.Context, state State) (State, error) {
		return state, nil
	})
	_ = g.AddNode("beta", func(ctx context.Context, state State) (State, error) {
		return state, nil
	})
	_ = g.AddEdge("alpha", "beta")
	_ = g.AddEdge("beta", EndNode)
	_ = g.SetStart("alpha")

	mw := func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			names = append(names, nodeName)
			return next(ctx, state)
		}
	}

	engine, err := NewEngine(g, EngineConfig{
		NodeMiddleware: []NodeMiddleware{mw},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Run(context.Background(), "test-2", NewState())
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("expected [alpha, beta], got %v", names)
	}
}

type workflowTenantKey struct{}

func TestEngine_NodeMiddleware_ContextPropagation(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode("check", func(ctx context.Context, state State) (State, error) {
		tenant, ok := ctx.Value(workflowTenantKey{}).(string)
		if !ok || tenant != "acme" {
			t.Fatalf("expected tenant 'acme' in node, got %q", tenant)
		}
		return state, nil
	})
	_ = g.AddEdge("check", EndNode)
	_ = g.SetStart("check")

	mw := func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			tenant, ok := ctx.Value(workflowTenantKey{}).(string)
			if !ok || tenant != "acme" {
				t.Fatalf("expected tenant 'acme' in middleware, got %q", tenant)
			}
			return next(ctx, state)
		}
	}

	engine, err := NewEngine(g, EngineConfig{
		NodeMiddleware: []NodeMiddleware{mw},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), workflowTenantKey{}, "acme")
	_, err = engine.Run(ctx, "test-3", NewState())
	if err != nil {
		t.Fatal(err)
	}
}

func TestEngine_NodeMiddleware_Resume(t *testing.T) {
	var visited []string

	store := NewMemoryCheckpointStore()

	g := NewGraph()
	_ = g.AddNode("first", func(ctx context.Context, state State) (State, error) {
		return Set(state, "done", true), nil
	})
	_ = g.AddNode("second", func(ctx context.Context, state State) (State, error) {
		visited = append(visited, "second")
		return state, nil
	})
	_ = g.AddEdge("first", "second")
	_ = g.AddEdge("second", EndNode)
	_ = g.SetStart("first")

	mw := func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			visited = append(visited, "mw:"+nodeName)
			return next(ctx, state)
		}
	}

	// Run to create a checkpoint
	engine, _ := NewEngine(g, EngineConfig{
		CheckpointStore: store,
		NodeMiddleware:  []NodeMiddleware{mw},
	})
	_, err := engine.Run(context.Background(), "resume-test", NewState())
	if err != nil {
		t.Fatal(err)
	}

	// Manually create a checkpoint at "first" to test Resume
	visited = nil
	_ = store.Save(context.Background(), Checkpoint{
		RunID:       "resume-test-2",
		State:       NewState(),
		CurrentNode: "first",
		Step:        1,
	})

	result, err := engine.Resume(context.Background(), "resume-test-2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %s: %s", result.Status, result.Error)
	}

	// Should have visited "second" with middleware
	foundMW := false
	foundNode := false
	for _, v := range visited {
		if v == "mw:second" {
			foundMW = true
		}
		if v == "second" {
			foundNode = true
		}
	}
	if !foundMW {
		t.Fatalf("expected middleware to wrap resumed node, visited: %v", visited)
	}
	if !foundNode {
		t.Fatalf("expected node to execute during resume, visited: %v", visited)
	}
}

