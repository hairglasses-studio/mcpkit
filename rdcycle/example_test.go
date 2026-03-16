package rdcycle_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/rdcycle"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ExampleNewModule demonstrates creating an rdcycle Module and registering it
// with a ToolRegistry so all R&D cycle tools become available.
func ExampleNewModule() {
	cfg := rdcycle.CycleConfig{
		RoadmapPath: "roadmap.json",
		GitRoot:     ".",
		ScanRepos:   []string{"anthropics/anthropic-sdk-go"},
		DateRange:   "2026-01-01",
	}

	m := rdcycle.NewModule(cfg)
	fmt.Println(m.Name())
	fmt.Println(len(m.Tools()), "tools registered")

	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)
	fmt.Println(reg.ToolCount(), "tools in registry")

	// Output:
	// rdcycle
	// 12 tools registered
	// 12 tools in registry
}

// ExampleNewInMemoryArtifactStore demonstrates saving and listing artifacts
// produced during an R&D cycle.
func ExampleNewInMemoryArtifactStore() {
	store := rdcycle.NewInMemoryArtifactStore()

	_ = store.Save(rdcycle.Artifact{
		ID:      "scan-001",
		Type:    "scan",
		Content: map[string]any{"repos": 3, "new_commits": 12},
	})
	_ = store.Save(rdcycle.Artifact{
		ID:      "plan-001",
		Type:    "plan",
		Content: map[string]any{"items": 5},
	})

	// Retrieve a specific artifact.
	a, ok := store.Get("scan-001")
	fmt.Println("found scan-001:", ok)
	fmt.Println("type:", a.Type)

	// List only scan artifacts.
	scans := store.List("scan")
	fmt.Println("scan count:", len(scans))

	// List all artifacts.
	all := store.List("")
	fmt.Println("total artifacts:", len(all))

	// Output:
	// found scan-001: true
	// type: scan
	// scan count: 1
	// total artifacts: 2
}

// ExamplePersonalProfile demonstrates the PersonalProfile budget preset
// for running R&D cycles under a Claude Max subscription.
func ExamplePersonalProfile() {
	p := rdcycle.PersonalProfile()

	fmt.Println("name:", p.Name)
	fmt.Printf("dollar budget: $%.0f/cycle\n", p.DollarBudget)
	fmt.Printf("daily cap:     $%.0f/day\n", p.DailyDollarCap)
	fmt.Println("max iterations:", p.MaxIterations)
	fmt.Println("model tiers:", len(p.ModelPricing))

	// BuildFinOpsStack composes the profile into tracker, cost policy, and windowed tracker.
	tracker, cp, wt := rdcycle.BuildFinOpsStack(p)
	fmt.Println("tracker ready:", tracker != nil)
	fmt.Println("cost policy ready:", cp != nil)
	fmt.Println("windowed tracker ready:", wt != nil)

	// Output:
	// name: personal
	// dollar budget: $5/cycle
	// daily cap:     $20/day
	// max iterations: 50
	// model tiers: 3
	// tracker ready: true
	// cost policy ready: true
	// windowed tracker ready: true
}
