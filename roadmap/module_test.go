package roadmap

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// writeTempRoadmap writes sample roadmap JSON to a temp directory and returns the path.
func writeTempRoadmap(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "roadmap.json")
	rm := sampleRoadmap()
	if err := SaveRoadmap(path, rm); err != nil {
		t.Fatalf("writeTempRoadmap: %v", err)
	}
	return path
}

func TestModuleInterface(t *testing.T) {
	m := NewModule()

	if m.Name() != "roadmap" {
		t.Errorf("Name = %q, want roadmap", m.Name())
	}
	if m.Description() == "" {
		t.Error("expected non-empty description")
	}

	tools := m.Tools()
	if len(tools) != 4 {
		t.Fatalf("tools count = %d, want 4", len(tools))
	}

	expectedNames := []string{
		"roadmap_read",
		"roadmap_update",
		"roadmap_gaps",
		"roadmap_next_phase",
	}
	for i, want := range expectedNames {
		if tools[i].Tool.Name != want {
			t.Errorf("tools[%d].Name = %q, want %q", i, tools[i].Tool.Name, want)
		}
		if tools[i].Category != "roadmap" {
			t.Errorf("tools[%d].Category = %q, want roadmap", i, tools[i].Category)
		}
	}

	// roadmap_update should be a write tool
	if !tools[1].IsWrite {
		t.Error("roadmap_update should have IsWrite=true")
	}
	// read tools should not be write
	if tools[0].IsWrite {
		t.Error("roadmap_read should have IsWrite=false")
	}
}

func TestNewModule_DefaultPath(t *testing.T) {
	m := NewModule()
	if m.roadmapPath != "roadmap.json" {
		t.Errorf("default path = %q, want roadmap.json", m.roadmapPath)
	}
}

func TestNewModule_CustomPath(t *testing.T) {
	m := NewModule(Config{RoadmapPath: "/custom/roadmap.json"})
	if m.roadmapPath != "/custom/roadmap.json" {
		t.Errorf("custom path = %q, want /custom/roadmap.json", m.roadmapPath)
	}
}

func TestRegistryIntegration(t *testing.T) {
	m := NewModule()
	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)

	srv := mcptest.NewServer(t, reg)

	expected := []string{
		"roadmap_read",
		"roadmap_update",
		"roadmap_gaps",
		"roadmap_next_phase",
	}
	for _, name := range expected {
		if !srv.HasTool(name) {
			t.Errorf("server missing tool: %s", name)
		}
	}
}

// TestHandleRead verifies the read handler returns roadmap data.
func TestHandleRead(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleRead(ctx, ReadInput{})
	if err != nil {
		t.Fatalf("handleRead: %v", err)
	}

	if out.Roadmap == nil {
		t.Fatal("expected non-nil roadmap")
	}
	if out.Roadmap.Title != "Test Roadmap" {
		t.Errorf("title = %q, want %q", out.Roadmap.Title, "Test Roadmap")
	}
	if len(out.Roadmap.Phases) != 3 {
		t.Errorf("phases = %d, want 3", len(out.Roadmap.Phases))
	}
	if out.Markdown == "" {
		t.Error("expected non-empty markdown")
	}
}

// TestHandleRead_PathOverride verifies the path parameter overrides the module default.
func TestHandleRead_PathOverride(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule() // default path is "roadmap.json" (would fail)
	ctx := context.Background()

	out, err := m.handleRead(ctx, ReadInput{Path: path})
	if err != nil {
		t.Fatalf("handleRead with path: %v", err)
	}
	if out.Roadmap.Title != "Test Roadmap" {
		t.Errorf("title = %q, want Test Roadmap", out.Roadmap.Title)
	}
}

// TestHandleRead_MissingFile verifies an error is returned for a missing file.
func TestHandleRead_MissingFile(t *testing.T) {
	m := NewModule(Config{RoadmapPath: "/nonexistent/roadmap.json"})
	ctx := context.Background()

	_, err := m.handleRead(ctx, ReadInput{})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestHandleUpdate_SetComplete verifies an item can be updated to complete.
func TestHandleUpdate_SetComplete(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleUpdate(ctx, UpdateInput{ItemID: "2C", Status: ItemStatusComplete})
	if err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if !out.Updated {
		t.Errorf("Updated = false, want true; message: %s", out.Message)
	}
	if out.ItemID != "2C" {
		t.Errorf("ItemID = %q, want 2C", out.ItemID)
	}
	if out.Status != ItemStatusComplete {
		t.Errorf("Status = %q, want complete", out.Status)
	}

	// Verify the change was persisted
	loaded, err := LoadRoadmap(path)
	if err != nil {
		t.Fatalf("LoadRoadmap after update: %v", err)
	}
	found := false
	for _, phase := range loaded.Phases {
		for _, item := range phase.Items {
			if item.ID == "2C" {
				if item.Status != ItemStatusComplete {
					t.Errorf("persisted status = %q, want complete", item.Status)
				}
				found = true
			}
		}
	}
	if !found {
		t.Error("item 2C not found in persisted roadmap")
	}
}

// TestHandleUpdate_NotFound verifies the handler returns Updated=false for unknown items.
func TestHandleUpdate_NotFound(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleUpdate(ctx, UpdateInput{ItemID: "Z999", Status: ItemStatusComplete})
	if err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if out.Updated {
		t.Error("expected Updated=false for unknown item")
	}
}

// TestHandleUpdate_InvalidStatus verifies validation on status field.
func TestHandleUpdate_InvalidStatus(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	_, err := m.handleUpdate(ctx, UpdateInput{ItemID: "2C", Status: "invalid"})
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

// TestHandleUpdate_EmptyItemID verifies validation on required item_id.
func TestHandleUpdate_EmptyItemID(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	_, err := m.handleUpdate(ctx, UpdateInput{Status: ItemStatusActive})
	if err == nil {
		t.Error("expected error for empty item_id")
	}
}

// TestHandleUpdate_TierItem verifies tier items can also be updated.
func TestHandleUpdate_TierItem(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleUpdate(ctx, UpdateInput{ItemID: "T1-1", Status: ItemStatusActive})
	if err != nil {
		t.Fatalf("handleUpdate tier item: %v", err)
	}
	if !out.Updated {
		t.Errorf("Updated = false for tier item; message: %s", out.Message)
	}
}

// TestHandleGaps verifies only planned items are returned.
func TestHandleGaps(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleGaps(ctx, GapsInput{})
	if err != nil {
		t.Fatalf("handleGaps: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("gap count = %d, want 3", out.Count)
	}
	if len(out.Gaps) != out.Count {
		t.Errorf("Gaps slice len = %d, Count = %d", len(out.Gaps), out.Count)
	}

	for _, g := range out.Gaps {
		if g.Status != ItemStatusPlanned {
			t.Errorf("gap item %q has status %q, want planned", g.ID, g.Status)
		}
	}
}

// TestHandleNextPhase verifies the next phase with ready items is returned.
func TestHandleNextPhase(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleNextPhase(ctx, NextPhaseInput{})
	if err != nil {
		t.Fatalf("handleNextPhase: %v", err)
	}
	if !out.Found {
		t.Fatal("expected Found=true")
	}
	if out.Phase == nil {
		t.Fatal("expected non-nil Phase")
	}
	if out.Phase.ID != "2" {
		t.Errorf("phase.ID = %q, want 2", out.Phase.ID)
	}
	// 2B and 2C depend on 2A (complete)
	if len(out.ReadyItems) == 0 {
		t.Error("expected non-empty ready items")
	}
}

// TestHandleNextPhase_AllComplete verifies Found=false when all phases are done.
func TestHandleNextPhase_AllComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roadmap.json")

	rm := &Roadmap{
		Title: "Done",
		Phases: []Phase{
			{ID: "1", Status: PhaseStatusComplete, Items: []WorkItem{}},
		},
	}
	if err := SaveRoadmap(path, rm); err != nil {
		t.Fatalf("save: %v", err)
	}

	m := NewModule(Config{RoadmapPath: path})
	ctx := context.Background()

	out, err := m.handleNextPhase(ctx, NextPhaseInput{})
	if err != nil {
		t.Fatalf("handleNextPhase: %v", err)
	}
	if out.Found {
		t.Error("expected Found=false when all phases complete")
	}
	if out.Phase != nil {
		t.Error("expected nil Phase when all phases complete")
	}
}

// TestHandleUpdate_PathOverride verifies the path parameter overrides the module default.
func TestHandleUpdate_PathOverride(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule() // default path would fail
	ctx := context.Background()

	out, err := m.handleUpdate(ctx, UpdateInput{ItemID: "2C", Status: ItemStatusComplete, Path: path})
	if err != nil {
		t.Fatalf("handleUpdate with path: %v", err)
	}
	if !out.Updated {
		t.Errorf("Updated = false, want true; message: %s", out.Message)
	}
}

// TestHandleUpdate_MissingFile verifies an error is returned when the file does not exist.
func TestHandleUpdate_MissingFile(t *testing.T) {
	m := NewModule(Config{RoadmapPath: "/nonexistent/roadmap.json"})
	ctx := context.Background()

	_, err := m.handleUpdate(ctx, UpdateInput{ItemID: "2C", Status: ItemStatusComplete})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestHandleGaps_PathOverride verifies the path parameter overrides the module default.
func TestHandleGaps_PathOverride(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule() // default path would fail
	ctx := context.Background()

	out, err := m.handleGaps(ctx, GapsInput{Path: path})
	if err != nil {
		t.Fatalf("handleGaps with path: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("gap count = %d, want 3", out.Count)
	}
}

// TestHandleGaps_MissingFile verifies an error is returned when the file does not exist.
func TestHandleGaps_MissingFile(t *testing.T) {
	m := NewModule(Config{RoadmapPath: "/nonexistent/roadmap.json"})
	ctx := context.Background()

	_, err := m.handleGaps(ctx, GapsInput{})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestHandleNextPhase_PathOverride verifies the path parameter overrides the module default.
func TestHandleNextPhase_PathOverride(t *testing.T) {
	path := writeTempRoadmap(t)
	m := NewModule() // default path would fail
	ctx := context.Background()

	out, err := m.handleNextPhase(ctx, NextPhaseInput{Path: path})
	if err != nil {
		t.Fatalf("handleNextPhase with path: %v", err)
	}
	if !out.Found {
		t.Error("expected Found=true")
	}
}

// TestHandleNextPhase_MissingFile verifies an error is returned when the file does not exist.
func TestHandleNextPhase_MissingFile(t *testing.T) {
	m := NewModule(Config{RoadmapPath: "/nonexistent/roadmap.json"})
	ctx := context.Background()

	_, err := m.handleNextPhase(ctx, NextPhaseInput{})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestGapAnalysis_TierPlannedItems verifies planned items in tiers are included.
func TestGapAnalysis_TierPlannedItems(t *testing.T) {
	t.Parallel()

	rm := &Roadmap{
		Tiers: []Tier{
			{
				ID:   "T1",
				Name: "Layer 1",
				Items: []WorkItem{
					{ID: "T1-1", Description: "planned tier item", Status: ItemStatusPlanned},
					{ID: "T1-2", Description: "complete tier item", Status: ItemStatusComplete},
				},
			},
		},
	}
	gaps := GapAnalysis(rm)
	if len(gaps) != 1 {
		t.Fatalf("gap count = %d, want 1", len(gaps))
	}
	if gaps[0].ID != "T1-1" {
		t.Errorf("gap ID = %q, want T1-1", gaps[0].ID)
	}
}

// TestSaveRoadmap_UnwritableDir verifies SaveRoadmap returns an error when it cannot create a temp file.
func TestSaveRoadmap_UnwritableDir(t *testing.T) {
	t.Parallel()

	// /nonexistent dir does not exist so CreateTemp will fail.
	rm := sampleRoadmap()
	err := SaveRoadmap("/nonexistent/dir/roadmap.json", rm)
	if err == nil {
		t.Error("expected error saving to non-existent directory")
	}
}
