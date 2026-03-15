package rdcycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/roadmap"
	"github.com/hairglasses-studio/mcpkit/workflow"
)

func TestNewRDCycleGraph_Valid(t *testing.T) {
	t.Parallel()
	rmPath := writeTestRoadmap(t)

	g, err := NewRDCycleGraph(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})
	if err != nil {
		t.Fatalf("NewRDCycleGraph: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestNewRDCycleGraph_NoRoadmapPath(t *testing.T) {
	t.Parallel()
	_, err := NewRDCycleGraph(CycleConfig{})
	if err == nil {
		t.Error("expected error for missing roadmap path")
	}
}

func TestRDCycleGraph_RunComplete(t *testing.T) {
	t.Parallel()
	rmPath := writeTestRoadmap(t)

	g, err := NewRDCycleGraph(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
		DateRange:   "2025-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("NewRDCycleGraph: %v", err)
	}

	engine, err := workflow.NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := engine.Run(context.Background(), "test-run", workflow.NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != workflow.RunStatusCompleted {
		t.Errorf("status = %v (err: %s); want completed", result.Status, result.Error)
	}

	// Verify state has expected data
	if v, ok := workflow.Get[bool](result.FinalState, "scan_complete"); !ok || !v {
		t.Error("expected scan_complete=true in final state")
	}
	if v, ok := workflow.Get[bool](result.FinalState, "implement_complete"); !ok || !v {
		t.Error("expected implement_complete=true in final state")
	}
}

func TestRDCycleGraph_EmptyPlanShortCircuit(t *testing.T) {
	t.Parallel()

	// Create a roadmap where all phases are complete
	rm := &roadmap.Roadmap{
		Title: "Test",
		Phases: []roadmap.Phase{
			{ID: "1", Name: "Done", Status: roadmap.PhaseStatusComplete, Items: []roadmap.WorkItem{
				{ID: "1a", Description: "done", Status: roadmap.ItemStatusComplete},
			}},
		},
	}
	dir := t.TempDir()
	rmPath := filepath.Join(dir, "roadmap.json")
	data, _ := json.MarshalIndent(rm, "", "  ")
	os.WriteFile(rmPath, data, 0644)

	g, err := NewRDCycleGraph(CycleConfig{
		RoadmapPath: rmPath,
	})
	if err != nil {
		t.Fatalf("NewRDCycleGraph: %v", err)
	}

	engine, err := workflow.NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := engine.Run(context.Background(), "empty-plan", workflow.NewState())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != workflow.RunStatusCompleted {
		t.Errorf("status = %v; want completed (short circuit)", result.Status)
	}

	// Should have gone through gate directly to END
	if _, ok := workflow.Get[bool](result.FinalState, "implement_complete"); ok {
		t.Error("implement should not have run with empty plan")
	}
}

func TestRDCycleGraph_Validation(t *testing.T) {
	t.Parallel()
	rmPath := writeTestRoadmap(t)

	g, err := NewRDCycleGraph(CycleConfig{
		RoadmapPath: rmPath,
	})
	if err != nil {
		t.Fatalf("NewRDCycleGraph: %v", err)
	}

	// Validate should pass (already called in constructor)
	if err := g.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// writeTestRoadmap creates a temporary roadmap file with active work items.
func writeTestRoadmap(t *testing.T) string {
	t.Helper()
	rm := &roadmap.Roadmap{
		Title: "Test Roadmap",
		Phases: []roadmap.Phase{
			{
				ID:     "20",
				Name:   "Test Phase",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "20A", Description: "Item A", Package: "pkg_a", Status: roadmap.ItemStatusPlanned},
					{ID: "20B", Description: "Item B", Package: "pkg_b", Status: roadmap.ItemStatusPlanned, DependsOn: []string{"20A"}},
				},
			},
		},
	}
	dir := t.TempDir()
	rmPath := filepath.Join(dir, "roadmap.json")
	data, err := json.MarshalIndent(rm, "", "  ")
	if err != nil {
		t.Fatalf("marshal roadmap: %v", err)
	}
	if err := os.WriteFile(rmPath, data, 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	return rmPath
}
