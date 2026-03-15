//go:build !official_sdk

package eval_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/eval"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// greetModule is a minimal ToolModule used in eval examples.
type greetModule struct{}

func (m *greetModule) Name() string        { return "greet" }
func (m *greetModule) Description() string { return "Greeting tool module" }
func (m *greetModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{Name: "greet", Description: "Greet"},
			Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
				return registry.MakeTextResult("hello world"), nil
			},
		},
	}
}

func ExampleRun() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&greetModule{})

	suite := eval.Suite{
		Name: "greet-suite",
		Cases: []eval.Case{
			{Name: "exact", Tool: "greet", Expected: "hello world"},
		},
		Scorers:   []eval.Scorer{eval.Contains()},
		Threshold: 1.0,
	}

	summary := eval.Run(context.Background(), suite, reg)
	fmt.Println(summary.PassRate())
	// Output:
	// 1
}

func ExampleExactMatch() {
	scorer := eval.ExactMatch()
	s := scorer.Score("hello", false, "hello")
	fmt.Println(s.Value)
	// Output:
	// 1
}
