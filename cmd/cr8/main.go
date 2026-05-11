package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hairglasses-studio/mcpkit/workflow"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cr8 [rip|analyze|enrich|sync]")
		os.Exit(1)
	}

	// Stub architecture: using mcpkit/workflow to structure the DJ crate pipeline.
	graph := workflow.NewGraph()
	graph.AddNode("rip", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		fmt.Println("Stage: Ripping SoundCloud likes...")
		return s, nil
	})
	graph.AddNode("analyze", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		fmt.Println("Stage: Analyzing BPM and Key...")
		return s, nil
	})
	graph.AddNode("enrich", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		fmt.Println("Stage: Enriching Metadata...")
		return s, nil
	})
	graph.AddNode("sync", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		fmt.Println("Stage: Syncing to Google Drive...")
		return s, nil
	})

	graph.AddEdge("rip", "analyze")
	graph.AddEdge("analyze", "enrich")
	graph.AddEdge("enrich", "sync")
	graph.SetStart("rip")

	engine, err := workflow.NewEngine(graph)
	if err != nil {
		fmt.Printf("Error creating workflow engine: %v\n", err)
		os.Exit(1)
	}

	result, err := engine.Run(context.Background(), "cr8-run-1", workflow.State{})
	if err != nil {
		fmt.Printf("Pipeline failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pipeline %s\n", result.Status)
}
