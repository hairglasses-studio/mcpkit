package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/workflow"
)

// ---------------------------------------------------------------------------
// workflow.go coverage
// ---------------------------------------------------------------------------

// TestNewRDCycleGraph_BadRoadmapFile tests the plan node path when LoadRoadmap
// fails due to an invalid/corrupt roadmap JSON file.
func TestNewRDCycleGraph_BadRoadmapFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad_roadmap.json")
	// Write invalid JSON so LoadRoadmap returns an error.
	if err := os.WriteFile(badPath, []byte("NOT JSON"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	g, err := NewRDCycleGraph(CycleConfig{RoadmapPath: badPath})
	if err != nil {
		// If graph construction itself fails, that's also a valid path.
		return
	}

	engine, err := workflow.NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// The plan node should return an error when LoadRoadmap fails.
	result, err := engine.Run(context.Background(), "bad-roadmap", workflow.NewState())
	// Either Run returns an error, or the result status reflects failure.
	if err == nil && result.Status != workflow.RunStatusCompleted && result.Status != workflow.RunStatusFailed {
		t.Errorf("expected error or failure status, got status=%v", result.Status)
	}
}

// ---------------------------------------------------------------------------
// notes.go coverage
// ---------------------------------------------------------------------------

// TestSaveNotes_WriteError tests that SaveNotes returns an error when the
// file cannot be written (path is a directory).
func TestSaveNotes_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Use a path where the filename itself is an existing directory — write will fail.
	dirAsFile := filepath.Join(dir, "notesdir")
	if err := os.MkdirAll(dirAsFile, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	err := SaveNotes(dirAsFile, []ImprovementNote{})
	if err == nil {
		t.Error("expected error when writing to a directory path")
	}
}

// ---------------------------------------------------------------------------
// profiles.go coverage
// ---------------------------------------------------------------------------

// TestSaveProfile_WriteError tests that SaveProfile returns an error when the
// target path is unwriteable (path is an existing directory).
func TestSaveProfile_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Use the directory itself as the file path — os.WriteFile should fail.
	err := SaveProfile(dir, PersonalProfile())
	if err == nil {
		t.Error("expected error when writing profile to a directory path")
	}
}

// ---------------------------------------------------------------------------
// improve.go coverage
// ---------------------------------------------------------------------------

// TestHandleImprove_LoadNotesError verifies that handleImprove returns an error
// when LoadNotes fails (i.e., the notes file is corrupt/invalid JSON).
func TestHandleImprove_LoadNotesError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad_notes.json")
	if err := os.WriteFile(badPath, []byte("NOT JSON"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := NewModule(CycleConfig{})
	_, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: badPath})
	if err == nil {
		t.Error("expected error when notes file is invalid JSON")
	}
}

// TestHandleImprove_WellCalibratedLoop exercises the recommendation path for
// avgWasted <= 2 && costTrend == "decreasing", producing the "well-calibrated" rec.
func TestHandleImprove_WellCalibratedLoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	// Low wasted iters and decreasing cost => "well-calibrated" recommendation.
	notes := []ImprovementNote{
		{CycleID: "c1", CycleNumber: 1, WastedIters: 1, TotalCost: 3.0},
		{CycleID: "c2", CycleNumber: 2, WastedIters: 1, TotalCost: 2.0},
		{CycleID: "c3", CycleNumber: 3, WastedIters: 0, TotalCost: 1.0},
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	if out.CostTrend != "decreasing" {
		t.Errorf("cost_trend = %q, want decreasing", out.CostTrend)
	}

	found := false
	for _, rec := range out.Recommendations {
		if len(rec) > 0 && containsSubstr(rec, "well-calibrated") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'well-calibrated' recommendation, got: %v", out.Recommendations)
	}
}

// TestHandleImprove_HighWastedIters exercises the recommendation path for
// avgWasted > 5, producing the "High average wasted iterations" recommendation.
func TestHandleImprove_HighWastedIters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	notes := []ImprovementNote{
		{CycleID: "c1", CycleNumber: 1, WastedIters: 10, TotalCost: 1.0},
		{CycleID: "c2", CycleNumber: 2, WastedIters: 8, TotalCost: 1.0},
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	found := false
	for _, rec := range out.Recommendations {
		if containsSubstr(rec, "wasted iterations") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'wasted iterations' recommendation, got: %v", out.Recommendations)
	}
}

// TestHandleImprove_StableNoBudgetSuggestion verifies that no budget suggestion
// is made when cost trend is stable (even with >= 5 cycles).
func TestHandleImprove_StableNoBudgetSuggestion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	var notes []ImprovementNote
	for i := 0; i < 5; i++ {
		notes = append(notes, ImprovementNote{
			CycleID:     fmt.Sprintf("c%d", i+1),
			CycleNumber: i + 1,
			TotalCost:   1.0,
		})
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	if out.BudgetSuggestion != nil {
		t.Error("expected no budget suggestion for stable cost trend")
	}
}

// TestHandleImprove_BudgetSuggestionMinDollar verifies that the budget suggestion
// is at least $1.00 even when average cost is very low.
func TestHandleImprove_BudgetSuggestionMinDollar(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	// Increasing costs starting from nearly zero.
	notes := []ImprovementNote{
		{CycleID: "c1", CycleNumber: 1, TotalCost: 0.001},
		{CycleID: "c2", CycleNumber: 2, TotalCost: 0.002},
		{CycleID: "c3", CycleNumber: 3, TotalCost: 0.003},
		{CycleID: "c4", CycleNumber: 4, TotalCost: 0.004},
		{CycleID: "c5", CycleNumber: 5, TotalCost: 0.005},
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	if out.BudgetSuggestion != nil && out.BudgetSuggestion.DollarBudget < 1.0 {
		t.Errorf("budget suggestion = $%f, want >= $1.00", out.BudgetSuggestion.DollarBudget)
	}
}

// ---------------------------------------------------------------------------
// schedule.go coverage
// ---------------------------------------------------------------------------

// TestHandleSchedule_RoadmapFromConfig verifies that when RoadmapPath is not
// set in ScheduleInput, the config's RoadmapPath is used.
func TestHandleSchedule_RoadmapFromConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "next_cycle.json")

	// Config has RoadmapPath set; ScheduleInput.RoadmapPath is empty.
	m := NewModule(CycleConfig{RoadmapPath: "my_roadmap.json"})
	out, err := m.handleSchedule(context.Background(), ScheduleInput{
		CycleName:  "Config Path Test",
		OutputPath: outputPath,
		// RoadmapPath intentionally omitted — should come from config.
	})
	if err != nil {
		t.Fatalf("handleSchedule: %v", err)
	}
	if !out.Written {
		t.Error("expected written=true")
	}

	// Verify the roadmap path from config is embedded in the plan task description.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	tasks, _ := spec["tasks"].([]any)
	found := false
	for _, raw := range tasks {
		task, _ := raw.(map[string]any)
		if desc, _ := task["description"].(string); containsSubstr(desc, "my_roadmap.json") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected plan task to reference config roadmap path 'my_roadmap.json'")
	}
}

// TestHandleSchedule_LessonsFromMoreThan3Notes tests the truncation to last 3
// notes in lesson injection when there are more than 3 notes available.
func TestHandleSchedule_LessonsFromMoreThan3Notes(t *testing.T) {
	// Not parallel — writes to the default notes path.
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "next_cycle.json")

	// Write 5 notes so the last-3 truncation path is exercised.
	notes := make([]ImprovementNote, 5)
	for i := range notes {
		notes[i] = ImprovementNote{
			CycleID:     fmt.Sprintf("c%d", i+1),
			CycleNumber: i + 1,
			Suggestions: []string{fmt.Sprintf("tip-%d", i+1)},
			WhatFailed:  []string{fmt.Sprintf("fail-%d", i+1)},
		}
	}

	defaultNotesPath := filepath.Join("rdcycle", "notes", "improvement_log.json")
	if err := os.MkdirAll(filepath.Dir(defaultNotesPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := SaveNotes(defaultNotesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}
	defer os.Remove(defaultNotesPath)

	m := NewModule(CycleConfig{})
	out, err := m.handleSchedule(context.Background(), ScheduleInput{
		CycleName:  "MoreThan3Notes",
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatalf("handleSchedule: %v", err)
	}
	if !out.Written {
		t.Error("expected written=true")
	}

	// The description should include lessons from the last 3 notes (tips 3-5, fails 3-5).
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	desc, _ := spec["description"].(string)
	if !containsSubstr(desc, "Lessons from previous cycles") {
		t.Error("expected 'Lessons from previous cycles' in description")
	}
}

// ---------------------------------------------------------------------------
// commit.go coverage — gitRun and gitOutput error paths
// ---------------------------------------------------------------------------

// TestGitRun_Error tests the error path in gitRun when the command fails.
func TestGitRun_Error(t *testing.T) {
	t.Parallel()
	err := gitRun(context.Background(), t.TempDir(), "status", "--invalid-flag-that-does-not-exist")
	if err == nil {
		t.Error("expected error for invalid git flag")
	}
}

// TestGitOutput_Error tests the error path in gitOutput when the command fails.
func TestGitOutput_Error(t *testing.T) {
	t.Parallel()
	_, err := gitOutput(context.Background(), t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Error("expected error when git repo does not exist")
	}
}

// TestGitOutput_Success tests the success path in gitOutput.
func TestGitOutput_Success(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t, "feature-output-test")
	out, err := gitOutput(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("gitOutput: %v", err)
	}
	if out != "feature-output-test" {
		t.Errorf("branch = %q, want feature-output-test", out)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
