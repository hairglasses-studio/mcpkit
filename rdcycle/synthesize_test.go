package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/ralph"
	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// staticSource is a test TaskSource that returns fixed candidates.
type staticSource struct {
	tasks []CandidateTask
	err   error
}

func (s *staticSource) Fetch(ctx context.Context) ([]CandidateTask, error) {
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

	// Should have: scan, plan, impl_session, verify, reflect, schedule
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

	// Should have scan + plan + 2 impl tasks + verify + reflect + schedule = 7
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

	// Maintenance should filter out roadmap source tasks.
	for _, task := range spec.Tasks {
		if task.ID == "impl_x" {
			t.Fatal("maintenance strategy should not include roadmap implementation tasks")
		}
	}
}

func TestSynthesize_AvoidPatterns(t *testing.T) {
	notesPath := filepath.Join(t.TempDir(), "notes.json")
	// Write notes with a recurring failure pattern.
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

	// The spec should pass ralph.ValidateSpec.
	if err := ralph.ValidateSpec(spec); err != nil {
		t.Fatalf("generated spec is invalid: %v", err)
	}
}

func TestRoadmapSource_Fetch(t *testing.T) {
	// Create a temp roadmap file.
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

	// item1 is ready (no deps), item2 blocked by item1, item3 is done.
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

func TestSynthesizeSpecBasic(t *testing.T) {
	ts := &TaskSynthesizer{}
	plan := PlanOutput{
		NextPhase: &roadmap.Phase{Name: "Phase 1", ID: "p1"},
		ReadyItems: []roadmap.WorkItem{
			{ID: "item-1", Description: "Build widget", Package: "widgets"},
			{ID: "item-2", Description: "Add tests"},
		},
		Suggestions: []string{
			"Focus on phase \"Phase 1\"",
			"Ecosystem signal: Review recent activity in owner/repo",
		},
	}

	spec, err := ts.SynthesizeSpec(plan, "cycle-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	if spec.Name != "R&D Cycle: cycle-1" {
		t.Errorf("unexpected name: %s", spec.Name)
	}

	// Should have: scan, plan, implement_item-1, implement_item-2, investigate_1, verify, reflect, report, schedule
	if len(spec.Tasks) != 9 {
		t.Errorf("expected 9 tasks, got %d", len(spec.Tasks))
		for _, task := range spec.Tasks {
			t.Logf("  task: %s", task.ID)
		}
	}

	// Verify DAG structure
	taskMap := make(map[string]ralph.Task)
	for _, task := range spec.Tasks {
		taskMap[task.ID] = task
	}

	if len(taskMap["scan"].DependsOn) != 0 {
		t.Error("scan should have no dependencies")
	}
	if taskMap["plan"].DependsOn[0] != "scan" {
		t.Error("plan should depend on scan")
	}
	if taskMap["implement_item-1"].DependsOn[0] != "plan" {
		t.Error("implement tasks should depend on plan")
	}

	// verify depends on all implement + investigate tasks
	verifyDeps := taskMap["verify"].DependsOn
	if len(verifyDeps) != 3 {
		t.Errorf("verify should depend on 3 tasks, got %d: %v", len(verifyDeps), verifyDeps)
	}
}

func TestSynthesizeSpecEmptyCycleName(t *testing.T) {
	ts := &TaskSynthesizer{}
	_, err := ts.SynthesizeSpec(PlanOutput{}, "", nil)
	if err == nil {
		t.Error("expected error for empty cycle name")
	}
}

func TestSynthesizeSpecNoReadyItems(t *testing.T) {
	ts := &TaskSynthesizer{}
	plan := PlanOutput{
		ReadyItems: nil,
	}

	spec, err := ts.SynthesizeSpec(plan, "empty-cycle", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should have generic implement task
	found := false
	for _, task := range spec.Tasks {
		if task.ID == "implement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected generic 'implement' task when no ready items")
	}
}

func TestSynthesizeSpecWithLessons(t *testing.T) {
	ts := &TaskSynthesizer{}
	lessons := []string{"Avoid flaky tests", "Use smaller batches"}

	spec, err := ts.SynthesizeSpec(PlanOutput{}, "lesson-cycle", lessons)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(spec.Description, "Avoid flaky tests") {
		t.Error("expected lessons in description")
	}
	if !strings.Contains(spec.Description, "Use smaller batches") {
		t.Error("expected all lessons in description")
	}
}

func TestWriteSpec(t *testing.T) {
	dir := t.TempDir()
	ts := &TaskSynthesizer{SpecDir: dir}

	spec := ralph.Spec{
		Name:       "Test Spec",
		Completion: "Done",
		Tasks:      []ralph.Task{{ID: "t1", Description: "do thing"}},
	}

	path, err := ts.WriteSpec(spec, "my-cycle")
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Dir(path) != dir {
		t.Errorf("expected path in %s, got %s", dir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded ralph.Spec
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Test Spec" {
		t.Errorf("expected loaded name to match, got %q", loaded.Name)
	}
}

func TestWriteSpecDefaultDir(t *testing.T) {
	// Use a temp working directory
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ts := &TaskSynthesizer{} // no SpecDir set
	spec := ralph.Spec{Name: "Default Dir Test", Tasks: []ralph.Task{{ID: "t1"}}}

	path, err := ts.WriteSpec(spec, "default-test")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(path, "rdcycle") {
		t.Errorf("expected default path to contain 'rdcycle', got %s", path)
	}
}

func TestSanitizeTaskID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"has space", "has_space"},
		{"path/sep", "path_sep"},
		{"Mixed.Case:Colon", "mixed_case_colon"},
	}
	for _, tt := range tests {
		got := sanitizeTaskID(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeTaskID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
