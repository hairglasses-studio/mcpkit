//go:build !official_sdk

package ralph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
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

func TestLoop_Hooks_IterationStartEnd(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var startCalls []int
	var endCalls []IterationLog

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		Hooks: Hooks{
			OnIterationStart: func(iteration int) {
				startCalls = append(startCalls, iteration)
			},
			OnIterationEnd: func(entry IterationLog) {
				endCalls = append(endCalls, entry)
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(startCalls) != 2 {
		t.Errorf("OnIterationStart called %d times, want 2", len(startCalls))
	}
	if len(startCalls) >= 2 && (startCalls[0] != 1 || startCalls[1] != 2) {
		t.Errorf("OnIterationStart calls = %v, want [1, 2]", startCalls)
	}
	if len(endCalls) != 2 {
		t.Errorf("OnIterationEnd called %d times, want 2", len(endCalls))
	}
}

func TestLoop_Hooks_TaskComplete(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var completedTasks []string

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		Hooks: Hooks{
			OnTaskComplete: func(taskID string) {
				completedTasks = append(completedTasks, taskID)
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(completedTasks) != 1 || completedTasks[0] != "t1" {
		t.Errorf("OnTaskComplete calls = %v, want [t1]", completedTasks)
	}
}

func TestLoop_Hooks_OnError(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "find tool"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "nonexistent", "arguments": {}}`,
			`{"complete": true, "reasoning": "give up"}`,
		},
	}

	reg := registry.NewToolRegistry()

	var errorCalls []int

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		Hooks: Hooks{
			OnError: func(iteration int, err error) {
				errorCalls = append(errorCalls, iteration)
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(errorCalls) != 1 || errorCalls[0] != 1 {
		t.Errorf("OnError calls = %v, want [1]", errorCalls)
	}
}

func TestLoop_Hooks_Nil(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "test spec", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()

	// No hooks set — verify no panics.
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
	// Should have logged the tool-not-found with available tools list.
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, `tool "nonexistent_tool" not found`) &&
			strings.Contains(entry.Result, "Available tools:") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool-not-found log entry with available tools list")
	}
}

// greetModule provides an additional "greet" tool for multi-tool tests.
type greetModule struct{}

func (m *greetModule) Name() string        { return "greet" }
func (m *greetModule) Description() string { return "Greet tools" }
func (m *greetModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{
				Name:        "greet",
				Description: "Returns a greeting",
			},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				args := registry.ExtractArguments(req)
				name, _ := args["name"].(string)
				return registry.MakeTextResult("hello: " + name), nil
			},
		},
	}
}

func TestLoop_MultiToolDecision(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "multi-tool test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "call two tools"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Decision uses tool_calls array with two tools.
	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_calls": [{"name": "echo", "arguments": {"message": "hi"}}, {"name": "greet", "arguments": {"name": "world"}}], "reasoning": "two tools", "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)
	reg.RegisterModule(&greetModule{})

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

	// Find the iteration log that has both tools.
	var multiEntry *IterationLog
	for i, entry := range status.Log {
		if len(entry.ToolCalls) == 2 {
			multiEntry = &status.Log[i]
			break
		}
	}
	if multiEntry == nil {
		t.Fatal("expected an iteration log with 2 tool calls")
	}
	if multiEntry.ToolCalls[0] != "echo" || multiEntry.ToolCalls[1] != "greet" {
		t.Errorf("ToolCalls = %v, want [echo greet]", multiEntry.ToolCalls)
	}
	if !strings.Contains(multiEntry.Result, "echo: hi") {
		t.Errorf("Result should contain echo result, got: %q", multiEntry.Result)
	}
	if !strings.Contains(multiEntry.Result, "hello: world") {
		t.Errorf("Result should contain greet result, got: %q", multiEntry.Result)
	}
}

func TestLoop_MultiToolPartialFailure(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "partial failure test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "one good one bad"}},
	}
	specFile := writeSpec(t, dir, spec)

	// One tool exists, one does not.
	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_calls": [{"name": "echo", "arguments": {"message": "ok"}}, {"name": "missing", "arguments": {}}], "reasoning": "partial"}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var errorCalls int
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		Hooks: Hooks{
			OnError: func(iteration int, err error) {
				errorCalls++
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if errorCalls != 1 {
		t.Errorf("OnError called %d times, want 1", errorCalls)
	}

	status := loop.Status()
	// Find the iteration with 2 tool calls.
	var partialEntry *IterationLog
	for i, entry := range status.Log {
		if len(entry.ToolCalls) == 2 {
			partialEntry = &status.Log[i]
			break
		}
	}
	if partialEntry == nil {
		t.Fatal("expected iteration log with 2 tool calls")
	}
	if !strings.Contains(partialEntry.Result, "echo: ok") {
		t.Errorf("Result should contain echo result, got: %q", partialEntry.Result)
	}
	if !strings.Contains(partialEntry.Result, `"missing" not found`) {
		t.Errorf("Result should contain not-found error, got: %q", partialEntry.Result)
	}
}

func TestLoop_CostTracking(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "cost tracking test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do something"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hello"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	tracker := finops.NewTracker()
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostTracker:  tracker,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	summary := tracker.Summary()
	if summary.TotalInvocations == 0 {
		t.Error("expected non-zero invocations in cost tracker")
	}
	totalTokens := summary.TotalInputTokens + summary.TotalOutputTokens
	if totalTokens == 0 {
		t.Error("expected non-zero total tokens in cost tracker")
	}
	if _, ok := summary.ByTool["ralph/sampling"]; !ok {
		t.Error("expected ralph/sampling entry in ByTool")
	}
	if _, ok := summary.ByTool["echo"]; !ok {
		t.Error("expected echo entry in ByTool")
	}
}

func TestLoop_ResumeFromProgress(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "resume test", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first"},
			{ID: "t2", Description: "second"},
		},
	}
	specFile := writeSpec(t, dir, spec)

	// Pre-write progress: iteration 1 done, t1 completed.
	progressFile := filepath.Join(dir, "spec.progress.json")
	progress := Progress{
		SpecFile:     specFile,
		Iteration:    1,
		CompletedIDs: []string{"t1"},
		Status:       StatusRunning,
		StartedAt:    time.Now(),
	}
	data, _ := json.Marshal(progress)
	os.WriteFile(progressFile, data, 0644)

	sampler := &scriptedSampler{
		responses: []string{
			// Should resume at iteration 2 — work on t2.
			`{"complete": false, "task_id": "t2", "tool_name": "echo", "arguments": {"message": "resumed"}, "mark_done": true}`,
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
	// Should have t1 from progress and t2 from this run.
	if len(status.CompletedIDs) != 2 {
		t.Errorf("CompletedIDs = %v, want 2 items", status.CompletedIDs)
	}
	// Iteration should be 3 (resumed from 1, ran 2 and 3).
	if status.Iteration != 3 {
		t.Errorf("Iteration = %d, want 3", status.Iteration)
	}
}

func TestLoop_CompletedNoRerun(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "completed test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Pre-write progress with StatusCompleted.
	progressFile := filepath.Join(dir, "spec.progress.json")
	progress := Progress{
		SpecFile:     specFile,
		Iteration:    2,
		CompletedIDs: []string{"t1"},
		Status:       StatusCompleted,
		StartedAt:    time.Now(),
	}
	data, _ := json.Marshal(progress)
	os.WriteFile(progressFile, data, 0644)

	// Sampler should never be called.
	sampler := &scriptedSampler{responses: []string{}}

	reg := registry.NewToolRegistry()

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run should return nil for completed loop, got: %v", err)
	}

	// Sampler should not have been called.
	if sampler.calls != 0 {
		t.Errorf("Sampler called %d times, want 0", sampler.calls)
	}
}

func TestLoop_ForceRestart(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "force restart test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Pre-write progress with StatusCompleted.
	progressFile := filepath.Join(dir, "spec.progress.json")
	progress := Progress{
		SpecFile:     specFile,
		Iteration:    5,
		CompletedIDs: []string{"t1"},
		Status:       StatusCompleted,
		StartedAt:    time.Now(),
	}
	data, _ := json.Marshal(progress)
	os.WriteFile(progressFile, data, 0644)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "fresh"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		ForceRestart: true,
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
	// Should start fresh — iteration count should be 2 (not 6+).
	if status.Iteration != 2 {
		t.Errorf("Iteration = %d, want 2 (fresh start)", status.Iteration)
	}
}

func TestLoop_WithTemplateVars(t *testing.T) {
	dir := t.TempDir()
	tmplFile := filepath.Join(dir, "spec.json")
	content := `{
		"name": "deploy-{{.Service}}",
		"description": "Deploy {{.Service}}",
		"completion": "deployed",
		"tasks": [{"id": "t1", "description": "Build {{.Service}}"}]
	}`
	os.WriteFile(tmplFile, []byte(content), 0644)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "building"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "deployed"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     tmplFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		TemplateVars: map[string]string{"Service": "api-gateway"},
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
}

func TestLoop_SkipsBlockedTask(t *testing.T) {
	dir := t.TempDir()
	// t2 depends on t1, so t2 is blocked until t1 is completed.
	spec := Spec{
		Name: "dag-test", Description: "DAG enforcement", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first task"},
			{ID: "t2", Description: "second task", DependsOn: []string{"t1"}},
		},
	}
	specFile := writeSpec(t, dir, spec)

	// LLM tries to target t2 first (blocked), then gives up.
	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t2", "tool_name": "echo", "arguments": {"message": "blocked"}, "reasoning": "try blocked"}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var echoCallCount int
	origReg := registry.NewToolRegistry()
	origReg.RegisterModule(&countingEchoModule{counter: &echoCallCount})

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: origReg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The tool should NOT have been executed for the blocked task.
	if echoCallCount != 0 {
		t.Errorf("echo called %d times, want 0 (task was blocked)", echoCallCount)
	}

	// The iteration log should record the blocked message with ready tasks hint.
	status := loop.Status()
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "blocked") && entry.TaskID == "t2" &&
			strings.Contains(entry.Result, "Ready tasks:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected blocked log entry for t2 with ready tasks hint, got log: %v", status.Log)
	}
}

func TestLoop_AllowsReadyTask(t *testing.T) {
	dir := t.TempDir()
	// t1 has no dependencies — it is immediately ready.
	spec := Spec{
		Name: "dag-ready", Description: "ready task", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first task"},
			{ID: "t2", Description: "second task", DependsOn: []string{"t1"}},
		},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "ready"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "done"}`,
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
	// t1 should be completed (it was ready and mark_done=true).
	found := false
	for _, id := range status.CompletedIDs {
		if id == "t1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("t1 not in CompletedIDs = %v", status.CompletedIDs)
	}
	// Verify echo actually ran (iteration log has tool call).
	var hasEcho bool
	for _, entry := range status.Log {
		for _, tc := range entry.ToolCalls {
			if tc == "echo" {
				hasEcho = true
			}
		}
	}
	if !hasEcho {
		t.Error("expected echo tool call in iteration log")
	}
}

func TestLoop_PreventMarkDoneOnBlockedTask(t *testing.T) {
	dir := t.TempDir()
	// t2 depends on t1 — blocked.
	spec := Spec{
		Name: "dag-markdone", Description: "mark done guard", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first task"},
			{ID: "t2", Description: "second task", DependsOn: []string{"t1"}},
		},
	}
	specFile := writeSpec(t, dir, spec)

	// Sampler tries to mark blocked t2 as done.
	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t2", "tool_name": "echo", "arguments": {"message": "sneak"}, "mark_done": true, "reasoning": "try mark blocked"}`,
			`{"complete": true, "reasoning": "give up"}`,
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
	for _, id := range status.CompletedIDs {
		if id == "t2" {
			t.Errorf("t2 should not appear in CompletedIDs (it was blocked), got: %v", status.CompletedIDs)
		}
	}
}

// countingEchoModule is a variant of echoModule that increments a counter on each call.
type countingEchoModule struct {
	counter *int
}

func (m *countingEchoModule) Name() string        { return "echo" }
func (m *countingEchoModule) Description() string { return "Counting echo" }
func (m *countingEchoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{
				Name:        "echo",
				Description: "Echoes the message back",
			},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				*m.counter++
				args := registry.ExtractArguments(req)
				msg, _ := args["message"].(string)
				return registry.MakeTextResult("echo: " + msg), nil
			},
		},
	}
}

func TestLoop_CostTrackingHook(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "cost hook test", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first"},
			{ID: "t2", Description: "second"},
		},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "one"}, "mark_done": true}`,
			`{"complete": false, "task_id": "t2", "tool_name": "echo", "arguments": {"message": "two"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	tracker := finops.NewTracker()
	var hookTotals []int64
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostTracker:  tracker,
		Hooks: Hooks{
			OnCostUpdate: func(iteration int, summary finops.UsageSummary) {
				hookTotals = append(hookTotals, summary.TotalInputTokens+summary.TotalOutputTokens)
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// OnCostUpdate should have been called twice (once per tool-executing iteration).
	if len(hookTotals) != 2 {
		t.Errorf("OnCostUpdate called %d times, want 2", len(hookTotals))
	}
	// Totals should be non-decreasing.
	if len(hookTotals) >= 2 && hookTotals[1] <= hookTotals[0] {
		t.Errorf("expected increasing totals, got %v", hookTotals)
	}
}

func TestLoop_HistoryWindow(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "history test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "first"}, "reasoning": "step 1"}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "second"}, "reasoning": "step 2", "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:      specFile,
		ToolRegistry:  reg,
		Sampler:       sampler,
		HistoryWindow: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify history was recorded.
	if len(loop.history) != 2 {
		t.Errorf("history length = %d, want 2 (one per tool-executing iteration)", len(loop.history))
	}

	// First turn should contain the first prompt and response.
	if len(loop.history) >= 1 {
		turn := loop.history[0]
		if turn.AssistantText == "" {
			t.Error("first turn AssistantText is empty")
		}
		if len(turn.ToolResults) == 0 {
			t.Error("first turn has no ToolResults")
		}
		if !strings.Contains(turn.ToolResults[0], "echo: first") {
			t.Errorf("first turn ToolResults[0] = %q, want contains 'echo: first'", turn.ToolResults[0])
		}
	}

	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", status.Status, StatusCompleted)
	}
}

func TestLoop_PhaseMaxTokens(t *testing.T) {
	// This test verifies that PhaseMaxTokens is used by checking that the sampler
	// receives different max tokens for different phases. We use a custom sampler
	// that records the request.
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "phase tokens test", Completion: "done",
		Tasks: []Task{{ID: "scan", Description: "scan phase"}},
	}
	specFile := writeSpec(t, dir, spec)

	var capturedMaxTokens []int
	sampler := &capturingScriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "scan", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "done"}`,
		},
		onRequest: func(req sampling.CreateMessageRequest) {
			capturedMaxTokens = append(capturedMaxTokens, req.MaxTokens)
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		MaxTokens:    4096,
		PhaseMaxTokens: map[string]int{
			"scan": 2048,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// First iteration should use scan's override of 2048.
	if len(capturedMaxTokens) == 0 {
		t.Fatal("no requests captured")
	}
	if capturedMaxTokens[0] != 2048 {
		t.Errorf("first request MaxTokens = %d, want 2048", capturedMaxTokens[0])
	}
}

// capturingScriptedSampler extends scriptedSampler with request capture.
type capturingScriptedSampler struct {
	responses []string
	calls     int
	onRequest func(sampling.CreateMessageRequest)
}

func (s *capturingScriptedSampler) CreateMessage(_ context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	if s.onRequest != nil {
		s.onRequest(req)
	}
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

func TestLoop_TaskDecomposer(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "decomposer test", Completion: "done",
		Tasks: []Task{
			{ID: "plan", Description: "plan phase"},
			{ID: "implement", Description: "implement phase", DependsOn: []string{"plan"}},
		},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			// Complete plan
			`{"complete": false, "task_id": "plan", "tool_name": "echo", "arguments": {"message": "planned"}, "mark_done": true, "reasoning": "plan done"}`,
			// Work on first sub-task
			`{"complete": false, "task_id": "impl_a", "tool_name": "echo", "arguments": {"message": "a"}, "mark_done": true, "reasoning": "impl_a done"}`,
			// Work on second sub-task
			`{"complete": false, "task_id": "impl_b", "tool_name": "echo", "arguments": {"message": "b"}, "mark_done": true, "reasoning": "impl_b done"}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var decomposerCalled bool
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		TaskDecomposer: func(taskID string, progress Progress, spec *Spec) []Task {
			if taskID == "plan" {
				decomposerCalled = true
				return []Task{
					{ID: "impl_a", Description: "implement file A", DependsOn: []string{"plan"}},
					{ID: "impl_b", Description: "implement file B", DependsOn: []string{"plan"}},
				}
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !decomposerCalled {
		t.Error("TaskDecomposer was not called")
	}

	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", status.Status, StatusCompleted)
	}
	// Should have plan, impl_a, impl_b completed.
	if len(status.CompletedIDs) != 3 {
		t.Errorf("CompletedIDs = %v, want 3 items", status.CompletedIDs)
	}
}

func TestBuildMessages(t *testing.T) {
	t.Parallel()
	history := []ConversationTurn{
		{UserPrompt: "prompt1", AssistantText: "response1", ToolResults: []string{"result1"}},
		{UserPrompt: "prompt2", AssistantText: "response2", ToolResults: []string{"result2a", "result2b"}},
		{UserPrompt: "prompt3", AssistantText: "response3", ToolResults: nil},
	}

	// Window of 2 should include only turns 2 and 3.
	messages := BuildMessages(history, 2, "current prompt")

	// Turn 2: user + assistant + tool results = 3 messages
	// Turn 3: user + assistant = 2 messages (no tool results)
	// Current prompt: 1 message
	// Total: 6 messages
	if len(messages) != 6 {
		t.Errorf("len(messages) = %d, want 6", len(messages))
		for i, m := range messages {
			content, _ := registry.ExtractTextContent(m.Content.(registry.Content))
			t.Logf("  [%d] role=%s text=%q", i, m.Role, content[:min(50, len(content))])
		}
	}

	// First message should be turn 2's user prompt.
	if len(messages) > 0 {
		text, _ := registry.ExtractTextContent(messages[0].Content.(registry.Content))
		if text != "prompt2" {
			t.Errorf("messages[0] text = %q, want 'prompt2'", text)
		}
	}

	// Last message should be current prompt.
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		text, _ := registry.ExtractTextContent(last.Content.(registry.Content))
		if text != "current prompt" {
			t.Errorf("last message text = %q, want 'current prompt'", text)
		}
	}
}

func TestBuildMessages_EmptyHistory(t *testing.T) {
	t.Parallel()
	messages := BuildMessages(nil, 5, "current prompt")
	if len(messages) != 1 {
		t.Errorf("len(messages) = %d, want 1 (just current prompt)", len(messages))
	}
}

func TestBuildMessages_WindowLargerThanHistory(t *testing.T) {
	t.Parallel()
	history := []ConversationTurn{
		{UserPrompt: "p1", AssistantText: "r1", ToolResults: []string{"t1"}},
	}
	messages := BuildMessages(history, 10, "current")
	// 1 turn (user + assistant + tool = 3) + current = 4
	if len(messages) != 4 {
		t.Errorf("len(messages) = %d, want 4", len(messages))
	}
}

func TestInjectSubTasks(t *testing.T) {
	tasks := []Task{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "replaced"},
		{ID: "c", Description: "third", DependsOn: []string{"b"}},
	}
	subTasks := []Task{
		{ID: "b1", Description: "sub 1", DependsOn: []string{"a"}},
		{ID: "b2", Description: "sub 2", DependsOn: []string{"a"}},
	}

	result := injectSubTasks(tasks, "b", subTasks)
	if len(result) != 4 {
		t.Fatalf("len(result) = %d, want 4", len(result))
	}
	if result[0].ID != "a" {
		t.Errorf("result[0].ID = %q, want 'a'", result[0].ID)
	}
	if result[1].ID != "b1" {
		t.Errorf("result[1].ID = %q, want 'b1'", result[1].ID)
	}
	if result[2].ID != "b2" {
		t.Errorf("result[2].ID = %q, want 'b2'", result[2].ID)
	}
	if result[3].ID != "c" {
		t.Errorf("result[3].ID = %q, want 'c'", result[3].ID)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Per-tool timeout tests (Commit 1) ---

// slowModule provides a tool that blocks until context is cancelled.
type slowModule struct{}

func (m *slowModule) Name() string        { return "slow" }
func (m *slowModule) Description() string { return "Slow tools" }
func (m *slowModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{Name: "slow_tool", Description: "Blocks for a while"},
			Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(5 * time.Second):
					return registry.MakeTextResult("slow done"), nil
				}
			},
		},
	}
}

func TestLoop_ToolTimeout(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "timeout test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "slow task"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "slow_tool", "arguments": {}, "reasoning": "slow"}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&slowModule{})

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		ToolTimeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The tool should have timed out, and the loop should have continued.
	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", status.Status, StatusCompleted)
	}
	// First iteration should have a tool error from timeout.
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "error") || strings.Contains(entry.Result, "deadline") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected timeout error in iteration log")
	}
}

func TestLoop_ToolTimeoutDefault(t *testing.T) {
	reg := registry.NewToolRegistry()
	echoTool(reg)

	dir := t.TempDir()
	spec := Spec{Name: "test", Description: "desc", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "x"}}}
	specFile := writeSpec(t, dir, spec)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      &scriptedSampler{responses: []string{`{"complete": true}`}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if loop.config.ToolTimeout != 30*time.Second {
		t.Errorf("default ToolTimeout = %v, want 30s", loop.config.ToolTimeout)
	}
}

// --- Sampler retry tests (Commit 2) ---

// failingSampler fails a specified number of times then delegates to a scriptedSampler.
type failingSampler struct {
	failCount  int
	calls      int
	inner      *scriptedSampler
}

func (s *failingSampler) CreateMessage(ctx context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	s.calls++
	if s.calls <= s.failCount {
		return nil, fmt.Errorf("transient error %d", s.calls)
	}
	return s.inner.CreateMessage(ctx, req)
}

func TestLoop_SamplerRetry_Recovers(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "retry test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	inner := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	// Fails twice, then succeeds. With SamplerRetries=2, the first call
	// will fail twice and succeed on the third attempt (attempt 0, 1, 2).
	sampler := &failingSampler{failCount: 2, inner: inner}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:       specFile,
		ToolRegistry:   reg,
		Sampler:        sampler,
		SamplerRetries: 2,
		SamplerBackoff: 10 * time.Millisecond, // fast for tests
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
	// No "sampler error" entries should be in the log (retry succeeded).
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "sampler error") {
			t.Errorf("unexpected sampler error in log: %s", entry.Result)
		}
	}
}

func TestLoop_SamplerRetry_Exhausted(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "retry exhausted", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "fail"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Always fails — inner sampler has no responses either.
	sampler := &failingSampler{failCount: 100, inner: &scriptedSampler{}}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:       specFile,
		ToolRegistry:   reg,
		Sampler:        sampler,
		SamplerRetries: 1,
		SamplerBackoff: 10 * time.Millisecond,
		MaxIterations:  2,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.Run(context.Background())
	// Should hit max iterations since every iteration exhausts retries.
	if err == nil {
		t.Fatal("expected error from max iterations")
	}

	status := loop.Status()
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "after 1 retries") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log entry with retry count")
	}
}

func TestLoop_SamplerRetry_RespectsStop(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "stop during retry", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "stoppable"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Always fails to trigger backoff.
	sampler := &failingSampler{failCount: 100, inner: &scriptedSampler{}}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:       specFile,
		ToolRegistry:   reg,
		Sampler:        sampler,
		SamplerRetries: 5,
		SamplerBackoff: 10 * time.Second, // long backoff
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- loop.Run(context.Background())
	}()

	// Give the loop time to start the first iteration and enter backoff.
	time.Sleep(100 * time.Millisecond)
	loop.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after Stop should return nil, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop within 5s")
	}

	status := loop.Status()
	if status.Status != StatusStopped {
		t.Errorf("Status = %q, want %q", status.Status, StatusStopped)
	}
}

// --- Budget guard tests (Commit 3) ---

func TestLoop_BudgetGuard_Stops(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "budget test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "expensive"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hello"}, "reasoning": "go"}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "again"}, "reasoning": "go"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	tracker := finops.NewTracker()
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostTracker:  tracker,
		BudgetLimit:  1, // extremely low — will trigger after first iteration
	})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.Run(context.Background())
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("error = %q, want contains 'budget exceeded'", err.Error())
	}

	status := loop.Status()
	if status.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", status.Status, StatusFailed)
	}
}

func TestLoop_BudgetGuard_ZeroUnlimited(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "no budget limit", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "cheap"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	tracker := finops.NewTracker()
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostTracker:  tracker,
		BudgetLimit:  0, // unlimited
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
}

// --- Parse error test (Commit 4) ---

func TestLoop_ParseError_ShowsRawText(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "parse error test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "parse fail"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			"this is not JSON at all, it's just random text from the LLM",
			`{"complete": true, "reasoning": "done"}`,
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
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "parse error") &&
			strings.Contains(entry.Result, "Raw response (truncated)") &&
			strings.Contains(entry.Result, "not JSON") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected parse error log with truncated raw response")
	}
}
