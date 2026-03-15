package rdcycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// writeRoadmap writes a Roadmap to a temp file and returns the path.
func writeRoadmap(t *testing.T, rm *roadmap.Roadmap) string {
	t.Helper()
	data, err := json.MarshalIndent(rm, "", "  ")
	if err != nil {
		t.Fatalf("marshal roadmap: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "roadmap.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	return path
}

func TestHandlePlan_NextPhase(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{
		Title: "Test",
		Phases: []roadmap.Phase{
			{
				ID:     "P1",
				Name:   "Phase One",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "1A", Description: "Do thing A", Status: roadmap.ItemStatusPlanned},
					{ID: "1B", Description: "Do thing B", Status: roadmap.ItemStatusComplete},
				},
			},
		},
	}

	path := writeRoadmap(t, rm)
	m := NewModule(CycleConfig{RoadmapPath: path})

	out, err := m.handlePlan(context.Background(), PlanInput{})
	if err != nil {
		t.Fatalf("handlePlan: unexpected error: %v", err)
	}
	if out.NextPhase == nil {
		t.Fatal("NextPhase: expected non-nil")
	}
	if out.NextPhase.ID != "P1" {
		t.Errorf("NextPhase.ID: want %q, got %q", "P1", out.NextPhase.ID)
	}
}

func TestHandlePlan_GapCount(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{
		Title: "Test",
		Phases: []roadmap.Phase{
			{
				ID:     "P1",
				Name:   "Phase One",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "1A", Status: roadmap.ItemStatusPlanned},
					{ID: "1B", Status: roadmap.ItemStatusPlanned},
					{ID: "1C", Status: roadmap.ItemStatusComplete},
				},
			},
		},
	}

	path := writeRoadmap(t, rm)
	m := NewModule(CycleConfig{RoadmapPath: path})

	out, err := m.handlePlan(context.Background(), PlanInput{})
	if err != nil {
		t.Fatalf("handlePlan: unexpected error: %v", err)
	}
	if out.GapCount != 2 {
		t.Errorf("GapCount: want 2, got %d", out.GapCount)
	}
}

func TestHandlePlan_ReadyItems(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{
		Title: "Test",
		Phases: []roadmap.Phase{
			{
				ID:     "P1",
				Name:   "Phase One",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "1A", Status: roadmap.ItemStatusPlanned},
					{ID: "1B", Status: roadmap.ItemStatusPlanned, DependsOn: []string{"1A"}},
				},
			},
		},
	}

	path := writeRoadmap(t, rm)
	m := NewModule(CycleConfig{RoadmapPath: path})

	out, err := m.handlePlan(context.Background(), PlanInput{})
	if err != nil {
		t.Fatalf("handlePlan: unexpected error: %v", err)
	}
	// 1A has no deps, so it's ready. 1B depends on 1A (not complete), so it's blocked.
	if len(out.ReadyItems) != 1 {
		t.Errorf("ReadyItems len: want 1, got %d", len(out.ReadyItems))
	}
	if out.ReadyItems[0].ID != "1A" {
		t.Errorf("ReadyItems[0].ID: want %q, got %q", "1A", out.ReadyItems[0].ID)
	}
}

func TestHandlePlan_ActionItemsInSuggestions(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{
		Title:  "Test",
		Phases: []roadmap.Phase{},
	}

	path := writeRoadmap(t, rm)
	m := NewModule(CycleConfig{RoadmapPath: path})

	out, err := m.handlePlan(context.Background(), PlanInput{
		ActionItems: []string{"New release in owner/repo"},
	})
	if err != nil {
		t.Fatalf("handlePlan: unexpected error: %v", err)
	}

	found := false
	for _, s := range out.Suggestions {
		if len(s) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Suggestions: expected at least one suggestion")
	}
}

func TestHandlePlan_PathOverride(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{
		Title: "Override",
		Phases: []roadmap.Phase{
			{
				ID:     "PX",
				Name:   "Override Phase",
				Status: roadmap.PhaseStatusPlanned,
				Items:  []roadmap.WorkItem{{ID: "XA", Status: roadmap.ItemStatusPlanned}},
			},
		},
	}

	path := writeRoadmap(t, rm)
	// Configure a non-existent default path; the input override should take precedence.
	m := NewModule(CycleConfig{RoadmapPath: "/nonexistent/roadmap.json"})

	out, err := m.handlePlan(context.Background(), PlanInput{RoadmapPath: path})
	if err != nil {
		t.Fatalf("handlePlan with path override: unexpected error: %v", err)
	}
	if out.NextPhase == nil || out.NextPhase.ID != "PX" {
		t.Errorf("NextPhase.ID: want %q, got %v", "PX", out.NextPhase)
	}
}

func TestHandlePlan_MissingRoadmap(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{RoadmapPath: "/nonexistent/roadmap.json"})

	_, err := m.handlePlan(context.Background(), PlanInput{})
	if err == nil {
		t.Error("handlePlan: expected error for missing roadmap file")
	}
}

func TestHandlePlan_ArtifactStored(t *testing.T) {
	t.Parallel()
	rm := &roadmap.Roadmap{Title: "Test", Phases: []roadmap.Phase{}}
	path := writeRoadmap(t, rm)
	m := NewModule(CycleConfig{RoadmapPath: path})

	out, err := m.handlePlan(context.Background(), PlanInput{})
	if err != nil {
		t.Fatalf("handlePlan: unexpected error: %v", err)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("artifact not stored")
	}
	if artifact.Type != "plan" {
		t.Errorf("artifact Type: want %q, got %q", "plan", artifact.Type)
	}
}

func TestBuildSuggestions_AllComplete(t *testing.T) {
	t.Parallel()
	suggestions := buildSuggestions(nil, nil, nil, nil)
	if len(suggestions) == 0 {
		t.Error("buildSuggestions: expected at least one suggestion when all complete")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input  string
		maxLen int
		wantOK bool // true = no truncation expected
	}{
		{"short", 10, true},
		{"exactly10!", 10, true},
		{"this is a long string that exceeds limit", 20, false},
	}
	for _, tc := range cases {
		got := truncate(tc.input, tc.maxLen)
		if tc.wantOK {
			if got != tc.input {
				t.Errorf("truncate(%q, %d) = %q; want unchanged", tc.input, tc.maxLen, got)
			}
		} else {
			if len(got) > tc.maxLen {
				t.Errorf("truncate(%q, %d) = %q; len %d > %d", tc.input, tc.maxLen, got, len(got), tc.maxLen)
			}
		}
	}
}
