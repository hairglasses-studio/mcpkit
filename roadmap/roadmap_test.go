package roadmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleRoadmap returns a test Roadmap for use in tests.
func sampleRoadmap() *Roadmap {
	return &Roadmap{
		Title:       "Test Roadmap",
		Description: "A test roadmap for unit tests",
		Phases: []Phase{
			{
				ID:     "1",
				Name:   "Foundation",
				Status: PhaseStatusComplete,
				Items: []WorkItem{
					{ID: "1A", Description: "Set up registry", Package: "registry", Status: ItemStatusComplete},
					{ID: "1B", Description: "Set up handler", Package: "handler", Status: ItemStatusComplete},
				},
			},
			{
				ID:     "2",
				Name:   "Core Features",
				Status: PhaseStatusActive,
				Items: []WorkItem{
					{ID: "2A", Description: "Add resilience", Package: "resilience", Status: ItemStatusComplete},
					{ID: "2B", Description: "Add auth", Package: "auth", Status: ItemStatusActive, DependsOn: []string{"2A"}},
					{ID: "2C", Description: "Add observability", Package: "observability", Status: ItemStatusPlanned, DependsOn: []string{"2A"}},
				},
			},
			{
				ID:     "3",
				Name:   "Advanced Features",
				Status: PhaseStatusPlanned,
				Items: []WorkItem{
					{ID: "3A", Description: "Add security", Package: "security", Status: ItemStatusPlanned},
					{ID: "3B", Description: "Add gateway", Package: "gateway", Status: ItemStatusPlanned, DependsOn: []string{"3A"}},
				},
			},
		},
		Tiers: []Tier{
			{
				ID:   "T1",
				Name: "Layer 1",
				Items: []WorkItem{
					{ID: "T1-1", Description: "registry", Package: "registry", Status: ItemStatusComplete},
				},
			},
		},
		UpdatedAt: "2026-03-15",
	}
}

// TestLoadSaveRoundTrip writes a roadmap to a temp file and reads it back.
func TestLoadSaveRoundTrip(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	dir := t.TempDir()
	path := filepath.Join(dir, "roadmap.json")

	if err := SaveRoadmap(path, rm); err != nil {
		t.Fatalf("SaveRoadmap: %v", err)
	}

	loaded, err := LoadRoadmap(path)
	if err != nil {
		t.Fatalf("LoadRoadmap: %v", err)
	}

	if loaded.Title != rm.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, rm.Title)
	}
	if loaded.Description != rm.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, rm.Description)
	}
	if len(loaded.Phases) != len(rm.Phases) {
		t.Fatalf("len(Phases) = %d, want %d", len(loaded.Phases), len(rm.Phases))
	}
	if len(loaded.Tiers) != len(rm.Tiers) {
		t.Fatalf("len(Tiers) = %d, want %d", len(loaded.Tiers), len(rm.Tiers))
	}

	// Verify phase content
	p2 := loaded.Phases[1]
	if p2.ID != "2" {
		t.Errorf("Phases[1].ID = %q, want %q", p2.ID, "2")
	}
	if len(p2.Items) != 3 {
		t.Fatalf("Phases[1] items = %d, want 3", len(p2.Items))
	}
	if p2.Items[2].Status != ItemStatusPlanned {
		t.Errorf("item 2C status = %q, want planned", p2.Items[2].Status)
	}
}

func TestLoadRoadmap_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoadmap(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadRoadmap_Missing(t *testing.T) {
	t.Parallel()

	_, err := LoadRoadmap("/tmp/nonexistent-roadmap-file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSaveRoadmap_AtomicWrite(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	dir := t.TempDir()
	path := filepath.Join(dir, "roadmap.json")

	if err := SaveRoadmap(path, rm); err != nil {
		t.Fatalf("SaveRoadmap: %v", err)
	}

	// Verify valid JSON was written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var check Roadmap
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if check.Title != rm.Title {
		t.Errorf("saved title = %q, want %q", check.Title, rm.Title)
	}
}

// TestLoadSaveRoundTrip_Markdown writes a roadmap to a temp markdown file and reads it back.
func TestLoadSaveRoundTrip_Markdown(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	// Add some extras to test full parsing
	rm.Phases[1].Items[1].DependsOn = []string{"2A"}
	rm.Phases[1].Items[1].Priority = "high"

	dir := t.TempDir()
	path := filepath.Join(dir, "ROADMAP.md")

	if err := SaveRoadmap(path, rm); err != nil {
		t.Fatalf("SaveRoadmap: %v", err)
	}

	loaded, err := LoadRoadmap(path)
	if err != nil {
		t.Fatalf("LoadRoadmap: %v", err)
	}

	if loaded.Title != rm.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, rm.Title)
	}
	if loaded.UpdatedAt != rm.UpdatedAt {
		t.Errorf("UpdatedAt = %q, want %q", loaded.UpdatedAt, rm.UpdatedAt)
	}
	if len(loaded.Phases) != len(rm.Phases) {
		t.Fatalf("len(Phases) = %d, want %d", len(loaded.Phases), len(rm.Phases))
	}

	// Verify phase 2 attributes
	p2 := loaded.Phases[1]
	if p2.ID != "2" {
		t.Errorf("Phases[1].ID = %q, want %q", p2.ID, "2")
	}
	if p2.Name != "Core Features" {
		t.Errorf("Phases[1].Name = %q, want %q", p2.Name, "Core Features")
	}
	if p2.Status != PhaseStatusActive {
		t.Errorf("Phases[1].Status = %q, want active", p2.Status)
	}

	// Verify item 2B (index 1 in phase 2)
	item2B := p2.Items[1]
	if item2B.ID != "2B" {
		t.Errorf("item ID = %q, want %q", item2B.ID, "2B")
	}
	if item2B.Priority != "high" {
		t.Errorf("item Priority = %q, want high", item2B.Priority)
	}
	if len(item2B.DependsOn) != 1 || item2B.DependsOn[0] != "2A" {
		t.Errorf("item DependsOn = %v, want [2A]", item2B.DependsOn)
	}
}

// TestRenderMarkdown_PhaseTag verifies phase XML tags appear in output.
func TestRenderMarkdown_PhaseTag(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	md := RenderMarkdown(rm)

	if !strings.Contains(md, "<roadmap-phase") {
		t.Error("expected <roadmap-phase> tag in markdown output")
	}
	if !strings.Contains(md, "</roadmap-phase>") {
		t.Error("expected </roadmap-phase> closing tag")
	}
}

// TestRenderMarkdown_ItemTag verifies item XML tags appear in output.
func TestRenderMarkdown_ItemTag(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	md := RenderMarkdown(rm)

	if !strings.Contains(md, "<roadmap-item") {
		t.Error("expected <roadmap-item> tag in markdown output")
	}
	if !strings.Contains(md, "</roadmap-item>") {
		t.Error("expected </roadmap-item> closing tag")
	}
}

// TestRenderMarkdown_TierTag verifies tier XML tags appear in output.
func TestRenderMarkdown_TierTag(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	md := RenderMarkdown(rm)

	if !strings.Contains(md, "<roadmap-tier") {
		t.Error("expected <roadmap-tier> tag in markdown output")
	}
	if !strings.Contains(md, "</roadmap-tier>") {
		t.Error("expected </roadmap-tier> closing tag")
	}
}

// TestRenderMarkdown_Content verifies key content appears in output.
func TestRenderMarkdown_Content(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	md := RenderMarkdown(rm)

	if !strings.Contains(md, "# Test Roadmap") {
		t.Error("expected title in markdown output")
	}
	if !strings.Contains(md, "Foundation") {
		t.Error("expected phase name 'Foundation' in markdown output")
	}
	if !strings.Contains(md, "Set up registry") {
		t.Error("expected item description in markdown output")
	}
	if !strings.Contains(md, "2026-03-15") {
		t.Error("expected updated_at in markdown output")
	}
}

// TestRenderMarkdown_StatusAttributes verifies status values appear in XML tags.
func TestRenderMarkdown_StatusAttributes(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	md := RenderMarkdown(rm)

	if !strings.Contains(md, `status="complete"`) {
		t.Error("expected status=complete in output")
	}
	if !strings.Contains(md, `status="active"`) {
		t.Error("expected status=active in output")
	}
	if !strings.Contains(md, `status="planned"`) {
		t.Error("expected status=planned in output")
	}
}

// TestNextPhase returns the first incomplete phase.
func TestNextPhase(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	phase := NextPhase(rm)

	if phase == nil {
		t.Fatal("expected non-nil phase")
	}
	if phase.ID != "2" {
		t.Errorf("NextPhase.ID = %q, want %q", phase.ID, "2")
	}
	if phase.Status != PhaseStatusActive {
		t.Errorf("NextPhase.Status = %q, want active", phase.Status)
	}
}

func TestNextPhase_AllComplete(t *testing.T) {
	t.Parallel()

	rm := &Roadmap{
		Phases: []Phase{
			{ID: "1", Status: PhaseStatusComplete},
			{ID: "2", Status: PhaseStatusComplete},
		},
	}
	phase := NextPhase(rm)
	if phase != nil {
		t.Errorf("expected nil, got phase ID %q", phase.ID)
	}
}

func TestNextPhase_EmptyRoadmap(t *testing.T) {
	t.Parallel()

	rm := &Roadmap{}
	phase := NextPhase(rm)
	if phase != nil {
		t.Error("expected nil for empty roadmap")
	}
}

// TestGapAnalysis returns only planned items.
func TestGapAnalysis(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	gaps := GapAnalysis(rm)

	// Phase 2: item 2C is planned
	// Phase 3: items 3A, 3B are planned
	// Tier T1: T1-1 is complete — should not appear
	if len(gaps) != 3 {
		t.Fatalf("gap count = %d, want 3", len(gaps))
	}

	ids := make(map[string]bool)
	for _, g := range gaps {
		ids[g.ID] = true
	}
	for _, expected := range []string{"2C", "3A", "3B"} {
		if !ids[expected] {
			t.Errorf("expected item %q in gaps", expected)
		}
	}
}

func TestGapAnalysis_Empty(t *testing.T) {
	t.Parallel()

	rm := &Roadmap{}
	gaps := GapAnalysis(rm)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(gaps))
	}
}

// TestReadyItems returns unblocked items within a phase.
func TestReadyItems(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	phase := &rm.Phases[1] // Phase 2: 2A complete, 2B active (dep 2A complete), 2C planned (dep 2A complete)

	ready := ReadyItems(phase)

	// 2B and 2C depend on 2A which is complete — both should be ready
	// 2B is active so it's not skipped by the complete check
	ids := make(map[string]bool)
	for _, r := range ready {
		ids[r.ID] = true
	}
	if !ids["2B"] {
		t.Error("expected item 2B to be ready (dep 2A complete)")
	}
	if !ids["2C"] {
		t.Error("expected item 2C to be ready (dep 2A complete)")
	}
	if ids["2A"] {
		t.Error("item 2A is complete, should not be in ready list")
	}
}

func TestReadyItems_NoDeps(t *testing.T) {
	t.Parallel()

	phase := &Phase{
		ID: "X",
		Items: []WorkItem{
			{ID: "X1", Status: ItemStatusPlanned},
			{ID: "X2", Status: ItemStatusPlanned},
		},
	}
	ready := ReadyItems(phase)
	if len(ready) != 2 {
		t.Errorf("ready count = %d, want 2", len(ready))
	}
}

func TestReadyItems_BlockedByDep(t *testing.T) {
	t.Parallel()

	phase := &Phase{
		ID: "X",
		Items: []WorkItem{
			{ID: "X1", Status: ItemStatusPlanned},
			{ID: "X2", Status: ItemStatusPlanned, DependsOn: []string{"X1"}},
		},
	}
	ready := ReadyItems(phase)

	ids := make(map[string]bool)
	for _, r := range ready {
		ids[r.ID] = true
	}
	if !ids["X1"] {
		t.Error("X1 has no deps, should be ready")
	}
	if ids["X2"] {
		t.Error("X2 depends on incomplete X1, should not be ready")
	}
}

func TestReadyItems_NilPhase(t *testing.T) {
	t.Parallel()

	ready := ReadyItems(nil)
	if ready != nil {
		t.Error("expected nil for nil phase")
	}
}

// TestPhaseByID looks up phases by ID.
func TestPhaseByID(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()

	phase := PhaseByID(rm, "2")
	if phase == nil {
		t.Fatal("expected non-nil phase for ID '2'")
	}
	if phase.Name != "Core Features" {
		t.Errorf("phase.Name = %q, want %q", phase.Name, "Core Features")
	}
}

func TestPhaseByID_Missing(t *testing.T) {
	t.Parallel()

	rm := sampleRoadmap()
	phase := PhaseByID(rm, "999")
	if phase != nil {
		t.Errorf("expected nil for missing ID, got phase %q", phase.ID)
	}
}

func TestPhaseByID_EmptyRoadmap(t *testing.T) {
	t.Parallel()

	rm := &Roadmap{}
	phase := PhaseByID(rm, "1")
	if phase != nil {
		t.Error("expected nil for empty roadmap")
	}
}
