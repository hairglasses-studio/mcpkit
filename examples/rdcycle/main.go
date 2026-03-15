//go:build !official_sdk

// Command rdcycle demonstrates the full R&D cycle: research + roadmap + rdcycle
// modules registered on a tool registry, wired into a workflow graph, and executed
// via ralph's WorkflowLoop.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hairglasses-studio/mcpkit/ralph"
	"github.com/hairglasses-studio/mcpkit/rdcycle"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/research"
	"github.com/hairglasses-studio/mcpkit/roadmap"
	"github.com/hairglasses-studio/mcpkit/workflow"
)

func main() {
	// 1. Create tool registry and register modules
	reg := registry.NewToolRegistry()

	researchMod := research.NewModule()
	reg.RegisterModule(researchMod)

	roadmapMod := roadmap.NewModule(roadmap.Config{
		RoadmapPath: "roadmap.json",
	})
	reg.RegisterModule(roadmapMod)

	cycleCfg := rdcycle.CycleConfig{
		RoadmapPath: "roadmap.json",
		GitRoot:     ".",
		ScanRepos:   []string{"mark3labs/mcp-go", "modelcontextprotocol/go-sdk"},
		DateRange:   "2025-03-01T00:00:00Z",
	}

	rdcycleMod := rdcycle.NewModule(cycleCfg)
	reg.RegisterModule(rdcycleMod)

	fmt.Printf("Registered %d tools from %d modules\n",
		len(reg.ListTools()), 3)

	// 2. Build the R&D cycle workflow graph
	g, err := rdcycle.NewRDCycleGraph(cycleCfg)
	if err != nil {
		log.Fatalf("Build R&D cycle graph: %v", err)
	}

	engine, err := workflow.NewEngine(g, workflow.EngineConfig{
		MaxSteps: 50,
	})
	if err != nil {
		log.Fatalf("Create workflow engine: %v", err)
	}

	// 3. Create a WorkflowLoop to bridge ralph lifecycle with the workflow
	wl, err := ralph.NewWorkflowLoop(ralph.WorkflowConfig{
		Engine:       engine,
		RunID:        "rdcycle-demo",
		InitialState: workflow.NewState(),
		Hooks: ralph.Hooks{
			OnIterationStart: func(i int) {
				fmt.Printf("[ralph] iteration %d starting\n", i)
			},
			OnIterationEnd: func(entry ralph.IterationLog) {
				fmt.Printf("[ralph] iteration %d: %s\n", entry.Iteration, entry.Result)
			},
			OnError: func(i int, err error) {
				fmt.Printf("[ralph] error at iteration %d: %v\n", i, err)
			},
		},
	})
	if err != nil {
		log.Fatalf("Create workflow loop: %v", err)
	}

	// 4. Run with graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Println("\nShutting down...")
		wl.Stop()
	}()

	fmt.Println("Starting R&D cycle workflow...")
	if err := wl.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Workflow failed: %v\n", err)
		os.Exit(1)
	}

	status := wl.Status()
	fmt.Printf("R&D cycle complete: status=%s iterations=%d\n", status.Status, status.Iteration)
}
