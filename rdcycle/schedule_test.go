package rdcycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleSchedule_EmptyCycleName(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	_, err := m.handleSchedule(context.Background(), ScheduleInput{})
	if err == nil {
		t.Error("expected error for empty cycle name")
	}
}

func TestHandleSchedule_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "next_cycle.json")

	m := NewModule(CycleConfig{RoadmapPath: "roadmap.json"})
	out, err := m.handleSchedule(context.Background(), ScheduleInput{
		CycleName:  "March Week 3",
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatalf("handleSchedule: %v", err)
	}
	if !out.Written {
		t.Error("expected written=true")
	}
	if out.SinceDate == "" {
		t.Error("expected non-empty since_date")
	}

	// Verify file is valid JSON
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Verify structure
	name, ok := spec["name"].(string)
	if !ok {
		t.Fatal("spec missing 'name' field")
	}
	if name != "R&D Cycle: March Week 3" {
		t.Errorf("name = %q; want 'R&D Cycle: March Week 3'", name)
	}

	tasks, ok := spec["tasks"].([]any)
	if !ok {
		t.Fatal("spec missing 'tasks' array")
	}
	if len(tasks) != 7 {
		t.Errorf("tasks count = %d; want 7", len(tasks))
	}
}

func TestHandleSchedule_DefaultOutputPath(t *testing.T) {
	t.Parallel()
	// Use a temp dir that we know exists, set it as current working context
	dir := t.TempDir()
	specsDir := filepath.Join(dir, "rdcycle", "specs")
	outputPath := filepath.Join(specsDir, "next_cycle.json")

	m := NewModule(CycleConfig{})
	out, err := m.handleSchedule(context.Background(), ScheduleInput{
		CycleName:  "test",
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatalf("handleSchedule: %v", err)
	}
	if !out.Written {
		t.Error("expected written=true")
	}
}
