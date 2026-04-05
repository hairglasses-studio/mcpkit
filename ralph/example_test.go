//go:build !official_sdk

package ralph_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/ralph"
)

func ExampleValidateSpec() {
	spec := ralph.Spec{
		Name:        "my-task",
		Description: "A simple automated task",
		Tasks: []ralph.Task{
			{ID: "t1", Description: "First step"},
			{ID: "t2", Description: "Second step", DependsOn: []string{"t1"}},
		},
	}

	err := ralph.ValidateSpec(spec)
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleDefaultProgressFile() {
	path := ralph.DefaultProgressFile("/tmp/my-spec.json")
	fmt.Println(path)
	// Output:
	// /tmp/my-spec.progress.json
}

func ExampleDecision_ResolvedToolCalls() {
	d := ralph.Decision{
		ToolName:  "search",
		Arguments: map[string]any{"query": "MCP"},
	}

	calls := d.ResolvedToolCalls()
	fmt.Println(len(calls))
	fmt.Println(calls[0].Name)
	// Output:
	// 1
	// search
}
