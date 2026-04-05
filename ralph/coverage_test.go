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

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
	"github.com/hairglasses-studio/mcpkit/workflow"
)

// --------------------------------------------------------------------------
// appendUnique — 75% coverage, missing "already in slice" branch
// --------------------------------------------------------------------------

func TestAppendUnique_AlreadyPresent(t *testing.T) {
	s := []string{"a", "b", "c"}
	result := appendUnique(s, "b")
	if len(result) != 3 {
		t.Errorf("appendUnique with existing item: len = %d, want 3", len(result))
	}
}

func TestAppendUnique_NewItem(t *testing.T) {
	s := []string{"a", "b"}
	result := appendUnique(s, "c")
	if len(result) != 3 {
		t.Errorf("appendUnique with new item: len = %d, want 3", len(result))
	}
	if result[2] != "c" {
		t.Errorf("appendUnique new item = %q, want %q", result[2], "c")
	}
}

// --------------------------------------------------------------------------
// parseDecision — 75.9% coverage, missing generic code-block path
// --------------------------------------------------------------------------

func TestParseDecision_GenericCodeBlock(t *testing.T) {
	// Generic ``` code block with no language tag
	input := "Here is the answer:\n```\n{\"complete\": true, \"reasoning\": \"done\"}\n```\n"
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision (generic code block): %v", err)
	}
	if !d.Complete {
		t.Error("expected Complete=true from generic code block")
	}
}

func TestParseDecision_GenericCodeBlockWithLangHint(t *testing.T) {
	// Generic ``` with a language hint that is not "json"
	input := "```go\n{\"complete\": false, \"tool_name\": \"echo\", \"arguments\": {}}\n```"
	d, err := parseDecision(input)
	if err != nil {
		t.Fatalf("parseDecision (go code block): %v", err)
	}
	if d.ToolName != "echo" {
		t.Errorf("ToolName = %q, want %q", d.ToolName, "echo")
	}
}

// --------------------------------------------------------------------------
// NewLoop — missing ToolRegistry nil and Sampler nil branches
// --------------------------------------------------------------------------

func TestNewLoop_NilToolRegistry(t *testing.T) {
	_, err := NewLoop(Config{
		SpecFile:     "spec.json",
		ToolRegistry: nil,
		Sampler:      &mockSampler{},
	})
	if err == nil {
		t.Fatal("expected error for nil ToolRegistry")
	}
	if !strings.Contains(err.Error(), "tool registry") {
		t.Errorf("error = %q, want mention of 'tool registry'", err)
	}
}

func TestNewLoop_NilSampler(t *testing.T) {
	_, err := NewLoop(Config{
		SpecFile:     "spec.json",
		ToolRegistry: registry.NewToolRegistry(),
		Sampler:      nil,
	})
	if err == nil {
		t.Fatal("expected error for nil Sampler")
	}
	if !strings.Contains(err.Error(), "sampler") {
		t.Errorf("error = %q, want mention of 'sampler'", err)
	}
}

// --------------------------------------------------------------------------
// LoadProgress — missing "parse error" branch
// --------------------------------------------------------------------------

func TestLoadProgress_ParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProgress(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON in progress file")
	}
	if !strings.Contains(err.Error(), "parse progress") {
		t.Errorf("error = %q, want mention of 'parse progress'", err)
	}
}

// --------------------------------------------------------------------------
// SaveProgress — missing CreateTemp failure path (non-writable dir)
// --------------------------------------------------------------------------

func TestSaveProgress_NonWritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: permission checks not enforced")
	}
	dir := t.TempDir()
	// Make dir non-writable so CreateTemp fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) }) //nolint:errcheck

	path := filepath.Join(dir, "progress.json")
	p := Progress{Status: StatusRunning}
	err := SaveProgress(path, p)
	if err == nil {
		t.Fatal("expected error writing to non-writable directory")
	}
}

// --------------------------------------------------------------------------
// renderSpecBytes — YAML validation failure path (80% coverage)
// --------------------------------------------------------------------------

func TestRenderSpecBytes_YAMLValidationFailure(t *testing.T) {
	// Valid YAML template that fails ValidateSpec (missing name)
	data := []byte(`
description: A spec without name
completion: done
tasks:
  - id: t1
    description: task one
`)
	_, err := renderSpecBytes(data, nil, ".yaml")
	if err == nil {
		t.Fatal("expected validation error for YAML spec with missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want mention of 'name is required'", err)
	}
}

func TestRenderSpecBytes_YAMLInvalidYAML(t *testing.T) {
	// Content that is syntactically invalid YAML
	data := []byte("name: [bad yaml\ndescription: broken\n  tasks: oops\n")
	_, err := renderSpecBytes(data, nil, ".yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML content")
	}
}

func TestRenderSpecBytes_JSONValidationFailure(t *testing.T) {
	// Valid JSON but fails ValidateSpec (missing description)
	data := []byte(`{"name": "test", "tasks": [{"id": "t1", "description": "do it"}]}`)
	_, err := renderSpecBytes(data, nil, "")
	if err == nil {
		t.Fatal("expected validation error for JSON spec with missing description")
	}
}

// --------------------------------------------------------------------------
// ParseSpecYAML — validation failure path (83.3% coverage)
// --------------------------------------------------------------------------

func TestParseSpecYAML_ValidationFailure(t *testing.T) {
	// Valid YAML but fails ValidateSpec (no tasks)
	yamlData := []byte(`
name: myspec
description: a spec
completion: done
tasks: []
`)
	_, err := ParseSpecYAML(yamlData)
	if err == nil {
		t.Fatal("expected validation error for YAML spec with no tasks")
	}
	if !strings.Contains(err.Error(), "at least one task") {
		t.Errorf("error = %q, want mention of 'at least one task'", err)
	}
}

// --------------------------------------------------------------------------
// Module.Tools — 33.3% coverage: ralph_start paths
// --------------------------------------------------------------------------

// blockingSampler blocks until its done channel is closed or context is cancelled.
type blockingSampler struct {
	done chan struct{}
}

func (s *blockingSampler) CreateMessage(ctx context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	select {
	case <-s.done:
	case <-ctx.Done():
	}
	return nil, fmt.Errorf("sampler blocked: stopped")
}

func TestModule_Start_Success(t *testing.T) {
	// Use os.MkdirTemp so we can control cleanup timing after stopping the loop.
	dir, err := os.MkdirTemp("", "ralph-module-start-*")
	if err != nil {
		t.Fatal(err)
	}

	spec := Spec{
		Name: "mod-test", Description: "module start test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	reg := registry.NewToolRegistry()
	doneCh := make(chan struct{})
	sampler := &blockingSampler{done: doneCh}

	m := NewModule(reg, sampler)
	tools := m.Tools()

	var startTool *registry.ToolDefinition
	for i := range tools {
		if tools[i].Tool.Name == "ralph_start" {
			startTool = &tools[i]
			break
		}
	}
	if startTool == nil {
		t.Fatal("ralph_start not found")
	}

	req := makeCallToolRequest("ralph_start", map[string]any{
		"spec_file": specFile,
	})

	result, callErr := startTool.Handler(context.Background(), req)
	if callErr != nil {
		t.Fatalf("ralph_start handler: %v", callErr)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	text := extractResultText(result)
	var out StartOutput
	if jsonErr := json.Unmarshal([]byte(text), &out); jsonErr != nil {
		t.Fatalf("parse StartOutput: %v — text: %s", jsonErr, text)
	}
	if out.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", out.Status, StatusRunning)
	}
	if out.Message != "loop started" {
		t.Errorf("Message = %q, want %q", out.Message, "loop started")
	}

	// Stop the background loop and wait for it to settle before cleaning up.
	close(doneCh)
	time.Sleep(100 * time.Millisecond)
	os.RemoveAll(dir)
}

func TestModule_Start_AlreadyRunning(t *testing.T) {
	dir, err := os.MkdirTemp("", "ralph-already-running-*")
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Name: "mod-running", Description: "already running test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	reg := registry.NewToolRegistry()
	doneCh := make(chan struct{})
	sampler := &blockingSampler{done: doneCh}
	defer func() {
		close(doneCh)
		time.Sleep(50 * time.Millisecond)
		os.RemoveAll(dir)
	}()

	m := NewModule(reg, sampler)
	tools := m.Tools()

	var startTool *registry.ToolDefinition
	for i := range tools {
		if tools[i].Tool.Name == "ralph_start" {
			startTool = &tools[i]
			break
		}
	}

	req := makeCallToolRequest("ralph_start", map[string]any{
		"spec_file": specFile,
	})

	// First start
	if _, err := startTool.Handler(context.Background(), req); err != nil {
		t.Fatalf("first ralph_start: %v", err)
	}

	// Give the loop goroutine time to start.
	time.Sleep(20 * time.Millisecond)

	// Second start — should return "already running"
	result, err := startTool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("second ralph_start: %v", err)
	}
	text := extractResultText(result)
	var out StartOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse StartOutput: %v — text: %s", err, text)
	}
	if out.Message != "loop is already running" {
		t.Errorf("Message = %q, want %q", out.Message, "loop is already running")
	}
}

func TestModule_Stop_WithLoop(t *testing.T) {
	dir, err := os.MkdirTemp("", "ralph-stop-*")
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Name: "mod-stop", Description: "stop test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	reg := registry.NewToolRegistry()
	doneCh := make(chan struct{})
	sampler := &blockingSampler{done: doneCh}
	defer func() {
		close(doneCh)
		time.Sleep(50 * time.Millisecond)
		os.RemoveAll(dir)
	}()

	m := NewModule(reg, sampler)
	tools := m.Tools()

	var startTool, stopTool *registry.ToolDefinition
	for i := range tools {
		switch tools[i].Tool.Name {
		case "ralph_start":
			startTool = &tools[i]
		case "ralph_stop":
			stopTool = &tools[i]
		}
	}

	startReq := makeCallToolRequest("ralph_start", map[string]any{
		"spec_file": specFile,
	})
	if _, err := startTool.Handler(context.Background(), startReq); err != nil {
		t.Fatalf("ralph_start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	stopReq := makeCallToolRequest("ralph_stop", map[string]any{})
	result, err := stopTool.Handler(context.Background(), stopReq)
	if err != nil {
		t.Fatalf("ralph_stop: %v", err)
	}
	text := extractResultText(result)
	var out StopOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse StopOutput: %v — text: %s", err, text)
	}
	if out.Message != "stop signal sent" {
		t.Errorf("Message = %q, want %q", out.Message, "stop signal sent")
	}
}

func TestModule_Status_WithLoop(t *testing.T) {
	dir, err := os.MkdirTemp("", "ralph-status-*")
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Name: "mod-status", Description: "status test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	reg := registry.NewToolRegistry()
	doneCh := make(chan struct{})
	sampler := &blockingSampler{done: doneCh}
	defer func() {
		close(doneCh)
		time.Sleep(50 * time.Millisecond) // let loop goroutine exit
		os.RemoveAll(dir)
	}()

	m := NewModule(reg, sampler)
	tools := m.Tools()

	var startTool, statusTool *registry.ToolDefinition
	for i := range tools {
		switch tools[i].Tool.Name {
		case "ralph_start":
			startTool = &tools[i]
		case "ralph_status":
			statusTool = &tools[i]
		}
	}

	startReq := makeCallToolRequest("ralph_start", map[string]any{
		"spec_file": specFile,
	})
	if _, err := startTool.Handler(context.Background(), startReq); err != nil {
		t.Fatalf("ralph_start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	statusReq := makeCallToolRequest("ralph_status", map[string]any{})
	result, err := statusTool.Handler(context.Background(), statusReq)
	if err != nil {
		t.Fatalf("ralph_status: %v", err)
	}
	text := extractResultText(result)
	var out StatusOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("parse StatusOutput: %v — text: %s", err, text)
	}
	if out.SpecFile != specFile {
		t.Errorf("SpecFile = %q, want %q", out.SpecFile, specFile)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — context cancel path
// --------------------------------------------------------------------------

// ctxCancelSampler blocks until context is cancelled, then returns an error.
type ctxCancelSampler struct{}

func (s *ctxCancelSampler) CreateMessage(ctx context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestLoop_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "ctx-test", Description: "context cancel test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &ctxCancelSampler{}
	reg := registry.NewToolRegistry()

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = loop.Run(ctx)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}

	status := loop.Status()
	if status.Status != StatusStopped {
		t.Errorf("Status = %q, want %q", status.Status, StatusStopped)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — sampler error path (continues to next iteration)
// --------------------------------------------------------------------------

// errorOnceSampler returns an error on the first call, then a scripted response.
type errorOnceSampler struct {
	calls      int
	afterError string
}

func (s *errorOnceSampler) CreateMessage(_ context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	s.calls++
	if s.calls == 1 {
		return nil, fmt.Errorf("intentional sampler error")
	}
	return &sampling.CreateMessageResult{
		SamplingMessage: registry.SamplingMessage{
			Content: registry.MakeTextContent(s.afterError),
			Role:    registry.RoleAssistant,
		},
	}, nil
}

func TestLoop_SamplerError(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "sampler-err", Description: "sampler error test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &errorOnceSampler{
		afterError: `{"complete": true, "reasoning": "done after error"}`,
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
}

// --------------------------------------------------------------------------
// Loop.Run — no tool specified (decision has no tool_name or tool_calls)
// --------------------------------------------------------------------------

func TestLoop_NoToolSpecified(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "no-tool", Description: "no tool decision", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Decision has neither tool_name nor tool_calls.
	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "reasoning": "thinking"}`,
			`{"complete": true}`,
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
}

// --------------------------------------------------------------------------
// Loop.Run — model selector
// --------------------------------------------------------------------------

func TestLoop_ModelSelector(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "model-select", Description: "model selector test", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}, "mark_done": true}`,
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	var selectedModels []string
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		ModelSelector: func(iteration int, completedIDs []string) string {
			model := "fast-model"
			if iteration > 1 {
				model = "smart-model"
			}
			selectedModels = append(selectedModels, model)
			return model
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(selectedModels) == 0 {
		t.Error("ModelSelector should have been called at least once")
	}
}

func TestLoop_ModelSelector_EmptyReturn(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "model-empty", Description: "model selector empty return", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		// ModelSelector returns empty string — should not add WithModel option
		ModelSelector: func(iteration int, completedIDs []string) string {
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// --------------------------------------------------------------------------
// WorkflowLoop — auto-generated RunID
// --------------------------------------------------------------------------

func TestNewWorkflowLoop_AutoRunID(t *testing.T) {
	e := buildSimpleGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		InitialState: workflow.NewState(),
		// RunID intentionally omitted — should be auto-generated.
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}
	if wl.config.RunID == "" {
		t.Error("expected auto-generated RunID, got empty string")
	}
	if !strings.HasPrefix(wl.config.RunID, "wf-") {
		t.Errorf("RunID = %q, want prefix 'wf-'", wl.config.RunID)
	}
}

// --------------------------------------------------------------------------
// WorkflowLoop.Run — RunStatusStopped path via context timeout
// --------------------------------------------------------------------------

func TestWorkflowLoop_StoppedStatus(t *testing.T) {
	e := buildSlowGraph(t)
	wl, err := NewWorkflowLoop(WorkflowConfig{
		Engine:       e,
		RunID:        "stopped-test",
		InitialState: workflow.NewState(),
	})
	if err != nil {
		t.Fatalf("NewWorkflowLoop: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = wl.Run(ctx)

	// Status should reflect a terminal state (failed or stopped).
	p := wl.Status()
	if p.Status != StatusFailed && p.Status != StatusStopped {
		t.Errorf("expected StatusFailed or StatusStopped, got %q", p.Status)
	}
}

// --------------------------------------------------------------------------
// ValidateSpec — missing "description is required" path
// --------------------------------------------------------------------------

func TestValidateSpec_MissingDescription(t *testing.T) {
	spec := Spec{
		Name:  "test",
		Tasks: []Task{{ID: "t1", Description: "first task"}},
	}
	err := ValidateSpec(spec)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("error = %q, want mention of 'description is required'", err)
	}
}

func TestValidateSpec_MissingTaskDescription(t *testing.T) {
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Tasks:       []Task{{ID: "t1"}}, // description missing
	}
	err := ValidateSpec(spec)
	if err == nil {
		t.Fatal("expected error for missing task description")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("error = %q, want mention of 'description is required'", err)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — ToolResult non-text content path
// --------------------------------------------------------------------------

// nonTextModule returns a tool that gives non-text content.
type nonTextModule struct{}

func (m *nonTextModule) Name() string        { return "nontext" }
func (m *nonTextModule) Description() string { return "Returns non-text content" }
func (m *nonTextModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{Name: "nontext", Description: "returns nothing useful"},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				// Return a result with no content at all.
				return &registry.CallToolResult{}, nil
			},
		},
	}
}

func TestLoop_ToolEmptyResult(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "empty-result", Description: "tool returns empty result", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "nontext", "arguments": {}}`,
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&nonTextModule{})

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
	// Find the log entry mentioning empty result.
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "tool returned empty result") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'tool returned empty result' log entry, got: %v", status.Log)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — ToolResult is error path
// --------------------------------------------------------------------------

// errorResultModule returns a tool that produces an IsError result.
type errorResultModule struct{}

func (m *errorResultModule) Name() string        { return "errtool" }
func (m *errorResultModule) Description() string { return "Returns an error result" }
func (m *errorResultModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{Name: "errtool", Description: "returns error content"},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				result := registry.MakeTextResult("something went wrong")
				result.IsError = true
				return result, nil
			},
		},
	}
}

func TestLoop_ToolErrorResult(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "err-result", Description: "tool returns error result", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "errtool", "arguments": {}}`,
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&errorResultModule{})

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
	// Find the entry with "tool error:"
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "tool error:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'tool error:' log entry, got: %v", status.Log)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — tool handler returns Go error
// --------------------------------------------------------------------------

// goErrModule returns a tool whose handler returns a Go error.
type goErrModule struct{}

func (m *goErrModule) Name() string        { return "goerr" }
func (m *goErrModule) Description() string { return "Returns a Go error" }
func (m *goErrModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: registry.Tool{Name: "goerr", Description: "returns go error"},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				return nil, fmt.Errorf("go-level tool error")
			},
		},
	}
}

func TestLoop_ToolGoError(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "go-err", Description: "tool handler returns go error", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do it"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "goerr", "arguments": {}}`,
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&goErrModule{})

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

	if errorCalls == 0 {
		t.Error("expected OnError to be called for go-level tool error")
	}
}

// --------------------------------------------------------------------------
// LoadSpec — JSON parse error
// --------------------------------------------------------------------------

func TestLoadSpec_JSONParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSpec(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON spec file")
	}
	if !strings.Contains(err.Error(), "parse spec") {
		t.Errorf("error = %q, want mention of 'parse spec'", err)
	}
}
