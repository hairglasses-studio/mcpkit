//go:build !official_sdk

package ralph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// scriptedSampler returns pre-configured responses in order.
type scriptedSampler struct {
	responses []string
	calls     int
}

func (s *scriptedSampler) CreateMessage(_ context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	if s.calls >= len(s.responses) {
		return nil, fmt.Errorf("no more scripted responses")
	}
	text := s.responses[s.calls]
	s.calls++
	return &sampling.CreateMessageResult{
		SamplingMessage: registry.SamplingMessage{
			Content: registry.MakeTextContent(text),
			Role:    registry.RoleAssistant,
		},
	}, nil
}

func writeSpec(t *testing.T, dir string, spec Spec) string {
	t.Helper()
	path := filepath.Join(dir, "spec.json")
	data, _ := json.Marshal(spec)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func echoTool(reg *registry.ToolRegistry) {
	reg.RegisterModule(&echoModule{})
}

type echoModule struct{}

func (m *echoModule) Name() string        { return "echo" }
func (m *echoModule) Description() string { return "Echo tools" }
func (m *echoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{
				Name:        "echo",
				Description: "Echoes the message back",
			},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				args := registry.ExtractArguments(req)
				msg, _ := args["message"].(string)
				return registry.MakeTextResult("echo: " + msg), nil
			},
		},
	}
}

func TestLoop_CompletesAllTasks(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hello"}, "reasoning": "testing", "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", status.Status, StatusCompleted)
	}
	if len(status.CompletedIDs) != 1 || status.CompletedIDs[0] != "t1" {
		t.Errorf("CompletedIDs = %v, want [t1]", status.CompletedIDs)
	}
}

func TestLoop_MaxIterations(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "never",
		Tasks: []Task{{ID: "t1", Description: "infinite"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Always returns a tool call, never completes.
	responses := make([]string, 5)
	for i := range responses {
		responses[i] = `{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "again"}, "reasoning": "keep going"}`
	}

	sampler := &scriptedSampler{responses: responses}
	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:      specFile,
		ToolRegistry:  reg,
		Sampler:       sampler,
		MaxIterations: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for max iterations")
	}

	status := loop.Status()
	if status.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", status.Status, StatusFailed)
	}
}

func TestLoop_Stop(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "never",
		Tasks: []Task{{ID: "t1", Description: "stoppable"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "once"}, "reasoning": "first"}`,
		},
	}
	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Stop before running — should exit immediately on next iteration check.
	loop.Stop()

	err = loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run after Stop should return nil, got: %v", err)
	}
	status := loop.Status()
	if status.Status != StatusStopped {
		t.Errorf("Status = %q, want %q", status.Status, StatusStopped)
	}
}

func TestLoop_ToolNotFound(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "find tool"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "nonexistent_tool", "arguments": {}, "reasoning": "try bad tool"}`,
			`{"complete": true, "reasoning": "give up"}`,
		},
	}
	reg := registry.NewToolRegistry()

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", status.Status, StatusCompleted)
	}
	// Should have logged the tool-not-found.
	found := false
	for _, entry := range status.Log {
		if entry.Result == `tool "nonexistent_tool" not found` {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool-not-found log entry")
	}
}
