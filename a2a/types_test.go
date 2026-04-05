package a2a

import (
	"encoding/json"
	"testing"
)

func TestTaskState_IsTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state    TaskState
		terminal bool
	}{
		{TaskSubmitted, false},
		{TaskWorking, false},
		{TaskInputNeeded, false},
		{TaskCompleted, true},
		{TaskCanceled, true},
		{TaskFailed, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestTextPart(t *testing.T) {
	t.Parallel()
	p := TextPart("hello")
	if p.Type != "text" {
		t.Errorf("Type = %q, want text", p.Type)
	}
	if p.Text != "hello" {
		t.Errorf("Text = %q, want hello", p.Text)
	}
}

func TestAgentCard_JSON(t *testing.T) {
	t.Parallel()
	card := AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "https://example.com/agent",
		Version:     "1.0.0",
		Provider:    &Provider{Organization: "Test Org"},
		Capabilities: &Capabilities{Streaming: true},
		Skills: []Skill{
			{ID: "greet", Name: "greet", Description: "Say hello"},
		},
	}
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AgentCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "test-agent" {
		t.Errorf("Name = %q, want test-agent", decoded.Name)
	}
	if len(decoded.Skills) != 1 {
		t.Errorf("Skills = %d, want 1", len(decoded.Skills))
	}
	if !decoded.Capabilities.Streaming {
		t.Error("Streaming should be true")
	}
}

func TestJSONRPCRequest_Marshal(t *testing.T) {
	t.Parallel()
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  json.RawMessage(`{"id":"task-1"}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty marshal output")
	}
}

func TestTask_FullLifecycle(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:    "task-1",
		State: TaskSubmitted,
		Messages: []Message{
			{Role: "user", Parts: []Part{TextPart("Fix the bug")}},
		},
	}
	if task.State.IsTerminal() {
		t.Error("submitted should not be terminal")
	}
	task.State = TaskWorking
	if task.State.IsTerminal() {
		t.Error("working should not be terminal")
	}
	task.State = TaskCompleted
	task.Artifacts = []Artifact{
		{Name: "fix.patch", Parts: []Part{TextPart("diff --git...")}, Index: 0, LastChunk: true},
	}
	if !task.State.IsTerminal() {
		t.Error("completed should be terminal")
	}
	if len(task.Artifacts) != 1 {
		t.Errorf("Artifacts = %d, want 1", len(task.Artifacts))
	}
}
