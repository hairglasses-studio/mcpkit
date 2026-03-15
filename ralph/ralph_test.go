package ralph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.json")

	spec := Spec{
		Name:        "test-task",
		Description: "A test task",
		Completion:  "All tasks done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task"},
		},
	}
	data, _ := json.Marshal(spec)
	os.WriteFile(specFile, data, 0644)

	loaded, err := LoadSpec(specFile)
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if loaded.Name != "test-task" {
		t.Errorf("Name = %q, want %q", loaded.Name, "test-task")
	}
	if len(loaded.Tasks) != 2 {
		t.Errorf("Tasks len = %d, want 2", len(loaded.Tasks))
	}
}

func TestLoadSpec_NotFound(t *testing.T) {
	_, err := LoadSpec("/nonexistent/spec.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSaveProgress_Atomic(t *testing.T) {
	dir := t.TempDir()
	progressFile := filepath.Join(dir, "progress.json")

	p := Progress{
		SpecFile:     "spec.json",
		Iteration:    3,
		CompletedIDs: []string{"t1"},
		Status:       StatusRunning,
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := SaveProgress(progressFile, p); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}

	loaded, err := LoadProgress(progressFile)
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if loaded.Iteration != 3 {
		t.Errorf("Iteration = %d, want 3", loaded.Iteration)
	}
	if len(loaded.CompletedIDs) != 1 || loaded.CompletedIDs[0] != "t1" {
		t.Errorf("CompletedIDs = %v, want [t1]", loaded.CompletedIDs)
	}
}

func TestLoadProgress_NotFound(t *testing.T) {
	p, err := LoadProgress("/nonexistent/progress.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if p.Iteration != 0 {
		t.Errorf("expected zero progress for missing file")
	}
}

func TestDefaultProgressFile(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"task.json", "task.progress.json"},
		{"path/to/spec.json", "path/to/spec.progress.json"},
		{"noext", "noext.progress.json"},
	}
	for _, tt := range tests {
		got := DefaultProgressFile(tt.input)
		if got != tt.want {
			t.Errorf("DefaultProgressFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDecision_ValidJSON(t *testing.T) {
	input := `{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"msg": "hi"}, "reasoning": "test", "mark_done": false}`
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision: %v", err)
	}
	if d.ToolName != "echo" {
		t.Errorf("ToolName = %q, want %q", d.ToolName, "echo")
	}
	if d.TaskID != "t1" {
		t.Errorf("TaskID = %q, want %q", d.TaskID, "t1")
	}
}

func TestParseDecision_Completion(t *testing.T) {
	input := `{"complete": true, "reasoning": "all done"}`
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision: %v", err)
	}
	if !d.Complete {
		t.Error("expected Complete=true")
	}
}

func TestParseDecision_CodeBlock(t *testing.T) {
	input := "Here's my decision:\n```json\n{\"complete\": true, \"reasoning\": \"done\"}\n```\n"
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision: %v", err)
	}
	if !d.Complete {
		t.Error("expected Complete=true from code block")
	}
}

func TestParseDecision_EmbeddedJSON(t *testing.T) {
	input := "I think we should do this: {\"complete\": false, \"tool_name\": \"search\", \"arguments\": {}} and that's it."
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision: %v", err)
	}
	if d.ToolName != "search" {
		t.Errorf("ToolName = %q, want %q", d.ToolName, "search")
	}
}

func TestParseDecision_Garbage(t *testing.T) {
	_, err := parseDecision("this is not json at all")
	if err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestBuildIterationPrompt(t *testing.T) {
	spec := Spec{
		Name:        "test",
		Description: "A test spec",
		Completion:  "All done",
		Tasks: []Task{
			{ID: "t1", Description: "First"},
			{ID: "t2", Description: "Second"},
		},
	}
	progress := Progress{
		CompletedIDs: []string{"t1"},
		Log: []IterationLog{
			{Iteration: 1, TaskID: "t1", ToolCalls: []string{"echo"}, Result: "ok"},
		},
	}

	reg := registry.NewToolRegistry()

	prompt := buildIterationPrompt(spec, progress, reg.GetAllToolDefinitions())
	if len(prompt) == 0 {
		t.Fatal("expected non-empty prompt")
	}
	// Check that completed task is marked
	if !contains(prompt, "[x] `t1`") {
		t.Error("expected t1 to be marked done")
	}
	if !contains(prompt, "[ ] `t2`") {
		t.Error("expected t2 to be not done")
	}
}

func TestNewLoop_Defaults(t *testing.T) {
	reg := registry.NewToolRegistry()
	sampler := &mockSampler{}

	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.json")
	spec := Spec{Name: "test", Tasks: []Task{{ID: "t1", Description: "do it"}}}
	data, _ := json.Marshal(spec)
	os.WriteFile(specFile, data, 0644)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatalf("NewLoop: %v", err)
	}
	if loop.config.MaxIterations != 100 {
		t.Errorf("MaxIterations = %d, want 100", loop.config.MaxIterations)
	}
	if loop.config.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %d, want 2048", loop.config.MaxTokens)
	}
}

func TestNewLoop_MissingRequired(t *testing.T) {
	_, err := NewLoop(Config{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockSampler is a minimal mock for tests that just need a non-nil sampler.
type mockSampler struct{}

func (m *mockSampler) CreateMessage(_ context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	return nil, fmt.Errorf("not implemented")
}
