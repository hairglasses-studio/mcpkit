package workflow

import (
	"context"
	"fmt"
)

// ExampleNewEngine demonstrates building a two-node linear workflow graph and
// running it through the engine to completion.
func ExampleNewEngine() {
	g := NewGraph()

	_ = g.AddNode("start", func(_ context.Context, s State) (State, error) {
		return Set(s, "visited_start", true), nil
	})
	_ = g.AddNode("finish", func(_ context.Context, s State) (State, error) {
		return Set(s, "done", true), nil
	})
	_ = g.AddEdge("start", "finish")
	_ = g.AddEdge("finish", EndNode)
	_ = g.SetStart("start")

	engine, err := NewEngine(g)
	if err != nil {
		fmt.Println("engine error:", err)
		return
	}

	result, err := engine.Run(context.Background(), "run-1", NewState())
	if err != nil {
		fmt.Println("run error:", err)
		return
	}
	fmt.Println("status:", result.Status)
	fmt.Println("steps:", result.Steps)
	// Output:
	// status: completed
	// steps: 2
}

// ExampleGraph_AddNode demonstrates building and validating a simple graph
// without running it.
func ExampleGraph_AddNode() {
	g := NewGraph()

	err := g.AddNode("process", func(_ context.Context, s State) (State, error) {
		return s, nil
	})
	fmt.Println("add node error:", err)

	_ = g.AddEdge("process", EndNode)
	_ = g.SetStart("process")

	err = g.Validate()
	fmt.Println("valid:", err == nil)
	// Output:
	// add node error: <nil>
	// valid: true
}
