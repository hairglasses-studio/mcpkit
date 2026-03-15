package orchestrator

import (
	"context"
	"fmt"
)

// ExampleFanOut demonstrates parallel execution of two stages that both
// receive the same input. The result contains one output per stage.
func ExampleFanOut() {
	double := StageFunc(func(_ context.Context, in StageInput) (*StageOutput, error) {
		v, _ := in.Data["value"].(int)
		return &StageOutput{Status: "ok", Data: map[string]any{"result": v * 2}}, nil
	})
	triple := StageFunc(func(_ context.Context, in StageInput) (*StageOutput, error) {
		v, _ := in.Data["value"].(int)
		return &StageOutput{Status: "ok", Data: map[string]any{"result": v * 3}}, nil
	})

	input := StageInput{Data: map[string]any{"value": 5}}
	result, err := FanOut(context.Background(), []StageFunc{double, triple}, input)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("pattern:", result.Pattern)
	fmt.Println("stages:", result.StageCount)
	// Output:
	// pattern: fan-out
	// stages: 2
}

// ExamplePipeline demonstrates sequential execution where each stage receives
// the merged output of the previous stage as its input.
func ExamplePipeline() {
	addTen := StageFunc(func(_ context.Context, in StageInput) (*StageOutput, error) {
		v, _ := in.Data["n"].(int)
		return &StageOutput{Status: "ok", Data: map[string]any{"n": v + 10}}, nil
	})
	double := StageFunc(func(_ context.Context, in StageInput) (*StageOutput, error) {
		v, _ := in.Data["n"].(int)
		return &StageOutput{Status: "ok", Data: map[string]any{"n": v * 2}}, nil
	})

	input := StageInput{Data: map[string]any{"n": 5}}
	result, err := Pipeline(context.Background(), []StageFunc{addTen, double}, input)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("pattern:", result.Pattern)
	fmt.Println("stages:", result.StageCount)
	n, _ := result.Outputs[1].Data["n"].(int)
	fmt.Println("final n:", n)
	// Output:
	// pattern: pipeline
	// stages: 2
	// final n: 30
}
