package rdcycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/ralph"
	"github.com/hairglasses-studio/mcpkit/roadmap"
)

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
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	ts := &TaskSynthesizer{}
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
