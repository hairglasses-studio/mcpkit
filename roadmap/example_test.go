package roadmap_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// sampleRM builds a minimal Roadmap for use in examples.
func sampleRM() *roadmap.Roadmap {
	return &roadmap.Roadmap{
		Title:       "My Project",
		Description: "Example roadmap",
		Phases: []roadmap.Phase{
			{
				ID:     "1",
				Name:   "Foundation",
				Status: roadmap.PhaseStatusComplete,
				Items: []roadmap.WorkItem{
					{ID: "1A", Description: "Set up registry", Status: roadmap.ItemStatusComplete},
				},
			},
			{
				ID:     "2",
				Name:   "Core Features",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "2A", Description: "Add auth", Status: roadmap.ItemStatusPlanned},
					{ID: "2B", Description: "Add observability", Status: roadmap.ItemStatusPlanned},
				},
			},
		},
	}
}

// ExampleLoadRoadmap shows how to write a roadmap to disk and load it back.
func ExampleLoadRoadmap() {
	rm := sampleRM()

	// Write to a temp file so the example is self-contained.
	dir, _ := os.MkdirTemp("", "roadmap-example-*")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "roadmap.json")
	data, _ := json.Marshal(rm)
	_ = os.WriteFile(path, data, 0644)

	loaded, err := roadmap.LoadRoadmap(path)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("title:", loaded.Title)
	fmt.Println("phases:", len(loaded.Phases))
	fmt.Println("first phase:", loaded.Phases[0].Name)

	// Output:
	// title: My Project
	// phases: 2
	// first phase: Foundation
}

// ExampleGapAnalysis shows how to find all work items that are still planned.
func ExampleGapAnalysis() {
	rm := sampleRM()

	gaps := roadmap.GapAnalysis(rm)
	fmt.Println("planned items:", len(gaps))
	for _, g := range gaps {
		fmt.Printf("  - %s: %s\n", g.ID, g.Description)
	}

	// Output:
	// planned items: 2
	//   - 2A: Add auth
	//   - 2B: Add observability
}

// ExampleNextPhase shows how to find the first incomplete phase so an autonomous
// agent knows where to focus next.
func ExampleNextPhase() {
	rm := sampleRM()

	phase := roadmap.NextPhase(rm)
	if phase == nil {
		fmt.Println("all phases complete")
		return
	}

	fmt.Println("next phase:", phase.Name)
	fmt.Println("status:", phase.Status)
	fmt.Println("items:", len(phase.Items))

	// Output:
	// next phase: Core Features
	// status: active
	// items: 2
}
