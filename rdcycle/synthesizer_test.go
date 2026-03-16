package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/ralph"
	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// staticSource is a test TaskSource that returns fixed candidates.
type staticSource struct {
	tasks []CandidateTask
	err   error
}

func (s *staticSource) Fetch(_ context.Context) ([]CandidateTask, error) {
	return s.tasks, s.err
}

func TestSynthesize_Basic(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "impl_session", Description: "Implement session package", Source: "roadmap", Priority: 10, Complexity: "moderate"},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "test-cycle",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyFull,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	if spec.Name == "" {
		t.Fatal("expected non-empty spec name")
	}

	taskIDs := make(map[string]bool)
	for _, task := range spec.Tasks {
		taskIDs[task.ID] = true
	}
	for _, expected := range []string{"scan", "plan", "impl_session", "verify", "reflect", "schedule"} {
		if !taskIDs[expected] {
			t.Errorf("expected task %q in spec", expected)
		}
	}
}

func TestSynthesize_EmptyName(t *testing.T) {
	le := NewLearningEngine(filepath.Join(t.TempDir(), "notes.json"))
	synth := NewSynthesizer(nil, le)
	_, err := synth.Synthesize(context.Background(), SynthesisConfig{})
	if err == nil {
		t.Fatal("expected error for empty cycle_name")
	}
}

func TestSynthesize_MaxTasks(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "t1", Description: "Task 1", Source: "roadmap", Priority: 1},
			{ID: "t2", Description: "Task 2", Source: "roadmap", Priority: 2},
			{ID: "t3", Description: "Task 3", Source: "roadmap", Priority: 3},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "max-test",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyFull,
		MaxTasks:    2,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	// scan + plan + 2 impl tasks + verify + reflect + schedule = 7
	if len(spec.Tasks) != 7 {
		t.Fatalf("expected 7 tasks (2 impl + 5 scaffold), got %d", len(spec.Tasks))
	}
}

func TestSynthesize_MaintenanceStrategy(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "impl_x", Description: "Implement X", Source: "roadmap", Priority: 10},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "maint",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyMaintenance,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	for _, task := range spec.Tasks {
		if task.ID == "impl_x" {
			t.Fatal("maintenance strategy should not include roadmap implementation tasks")
		}
	}
}

func TestSynthesize_AvoidPatterns(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"session timeout"}},
		{CycleID: "c2", WhatFailed: []string{"session timeout"}},
		{CycleID: "c3", WhatFailed: []string{"session timeout"}},
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatal(err)
	}
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "session_impl", Description: "Fix session timeout handling", Source: "roadmap", Priority: 10},
			{ID: "other_impl", Description: "Implement other feature", Source: "roadmap", Priority: 20},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "avoid-test",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyFull,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	for _, task := range spec.Tasks {
		if task.ID == "session_impl" {
			t.Fatal("expected session_impl to be filtered by avoid pattern")
		}
	}
}

func TestSynthesize_FailedSourceContinues(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	failing := &staticSource{err: fmt.Errorf("source error")}
	working := &staticSource{
		tasks: []CandidateTask{
			{ID: "t1", Description: "Task 1", Source: "roadmap", Priority: 10},
		},
	}

	synth := NewSynthesizer([]TaskSource{failing, working}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "fail-test",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyFull,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	taskIDs := make(map[string]bool)
	for _, task := range spec.Tasks {
		taskIDs[task.ID] = true
	}
	if !taskIDs["t1"] {
		t.Fatal("expected t1 from working source")
	}
}

func TestSynthesize_RecoveryStrategy(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "roadmap_task", Description: "Roadmap task", Source: "roadmap", Priority: 10},
			{ID: "fix_task", Description: "Fix issue", Source: "improvement", Priority: 5},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName: "recovery",
		Strategy:  StrategyRecovery,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range spec.Tasks {
		if task.ID == "roadmap_task" {
			t.Fatal("recovery should filter out roadmap tasks")
		}
	}
}

func TestSynthesize_ValidSpec(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	le := NewLearningEngine(notesPath)

	src := &staticSource{
		tasks: []CandidateTask{
			{ID: "impl_a", Description: "Implement A", Source: "roadmap", Priority: 10},
		},
	}

	synth := NewSynthesizer([]TaskSource{src}, le)
	spec, err := synth.Synthesize(context.Background(), SynthesisConfig{
		CycleName:   "valid-test",
		RoadmapPath: "roadmap.json",
		Strategy:    StrategyFull,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ralph.ValidateSpec(spec); err != nil {
		t.Fatalf("generated spec is invalid: %v", err)
	}
}

func TestRoadmapSource_Fetch(t *testing.T) {
	dir := t.TempDir()
	rmPath := filepath.Join(dir, "roadmap.json")
	rm := roadmap.Roadmap{
		Title: "Test",
		Phases: []roadmap.Phase{
			{
				ID:     "p1",
				Name:   "Phase 1",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "item1", Description: "First item", Status: roadmap.ItemStatusPlanned, Priority: "high"},
					{ID: "item2", Description: "Second item", Status: roadmap.ItemStatusPlanned, Priority: "low", DependsOn: []string{"item1"}},
					{ID: "item3", Description: "Done item", Status: roadmap.ItemStatusComplete},
				},
			},
		},
	}
	data, _ := json.Marshal(rm)
	os.WriteFile(rmPath, data, 0644)

	src := NewRoadmapSource(rmPath)
	tasks, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(tasks))
	}
	if tasks[0].ID != "item1" {
		t.Fatalf("expected item1, got %s", tasks[0].ID)
	}
	if tasks[0].Complexity != "complex" {
		t.Fatalf("expected complex for high priority, got %s", tasks[0].Complexity)
	}
}

func TestImprovementSource_FetchEmpty(t *testing.T) {
	src := NewImprovementSource(filepath.Join(t.TempDir(), "nonexistent.json"))
	tasks, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestImprovementSource_SelfImproveAt10(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	var notes []ImprovementNote
	for i := 0; i < 10; i++ {
		notes = append(notes, ImprovementNote{CycleID: fmt.Sprintf("c%d", i)})
	}
	SaveNotes(notesPath, notes)

	src := NewImprovementSource(notesPath)
	tasks, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var foundImprove bool
	for _, task := range tasks {
		if task.ID == "self_improve" {
			foundImprove = true
		}
	}
	if !foundImprove {
		t.Fatal("expected self_improve task at 10 cycles")
	}
}

func TestImprovementSource_FixTasks(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"network error"}},
		{CycleID: "c2", WhatFailed: []string{"network error"}},
		{CycleID: "c3", WhatFailed: []string{"other"}},
	}
	SaveNotes(notesPath, notes)

	src := NewImprovementSource(notesPath)
	tasks, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var foundFix bool
	for _, task := range tasks {
		if task.Source == "improvement" && task.Complexity == "moderate" {
			foundFix = true
		}
	}
	if !foundFix {
		t.Fatal("expected fix task for recurring failure")
	}
}
