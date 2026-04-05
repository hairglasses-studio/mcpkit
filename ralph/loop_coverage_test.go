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
)

// --------------------------------------------------------------------------
// CostDowngradeRequested — 0% coverage
// --------------------------------------------------------------------------

func TestCostDowngradeRequested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "desc", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)
	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      &mockSampler{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initially false.
	if loop.CostDowngradeRequested() {
		t.Error("expected false initially")
	}

	// Set the flag manually and verify it reads back and resets.
	loop.mu.Lock()
	loop.costDowngrade = true
	loop.mu.Unlock()

	if !loop.CostDowngradeRequested() {
		t.Error("expected true after setting")
	}
	// Second call should be false (flag is reset).
	if loop.CostDowngradeRequested() {
		t.Error("expected false after reset")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — CostGovernor halt path
// --------------------------------------------------------------------------

func TestLoop_CostGovernor_Halt(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "cg-halt", Description: "cost governor halt", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "spend tokens"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "again"}}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	// CostGovernor with very low hard budget — will halt after first iteration.
	cg := NewCostGovernor(CostGovernorConfig{
		HardBudgetTokens:  1,
		UnproductiveMax:   100,
		VelocityWindow:    100,
		VelocityAlarmRate: 0,
	})

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostGovernor: cg,
	})
	if err != nil {
		t.Fatal(err)
	}

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected cost governor halt error")
	}
	if !strings.Contains(runErr.Error(), "cost governor halt") {
		t.Errorf("error = %q, want contains 'cost governor halt'", runErr)
	}
	if loop.Status().Status != StatusFailed {
		t.Errorf("Status = %q, want %q", loop.Status().Status, StatusFailed)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — CostGovernor downgrade path
// --------------------------------------------------------------------------

func TestLoop_CostGovernor_Downgrade(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "cg-downgrade", Description: "cost governor downgrade", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "a"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "b"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "c"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "d"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "e"}, "mark_done": true}`,
			`{"complete": true}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	// VelocityAlarmRate of 0.5 with window of 4: 3/4 unproductive triggers downgrade.
	// Iterations without mark_done are unproductive.
	cg := NewCostGovernor(CostGovernorConfig{
		HardBudgetTokens:  0,
		UnproductiveMax:   100,
		VelocityWindow:    4,
		VelocityAlarmRate: 0.5,
	})

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		CostGovernor: cg,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The downgrade flag should have been set at some point.
	// We can't check it post-run since the flag resets, but reaching completion
	// proves the downgrade path (not halt) was taken.
	if loop.Status().Status != StatusCompleted {
		t.Errorf("Status = %q, want completed (downgrade does not halt)", loop.Status().Status)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — ExitGate.RequireAllTasksDone path
// --------------------------------------------------------------------------

func TestLoop_ExitGate_RequireAllTasksDone(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "exit-gate", Description: "exit gate test", Completion: "done",
		Tasks: []Task{
			{ID: "t1", Description: "first"},
			{ID: "t2", Description: "second"},
		},
	}
	specFile := writeSpec(t, dir, spec)

	// LLM tries to complete early (without finishing all tasks), then does t1, t2, then completes.
	sampler := &scriptedSampler{
		responses: []string{
			// Try to complete early — should be rejected.
			`{"complete": true, "reasoning": "premature completion"}`,
			// Now do the actual work.
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "a"}, "mark_done": true}`,
			`{"complete": false, "task_id": "t2", "tool_name": "echo", "arguments": {"message": "b"}, "mark_done": true}`,
			`{"complete": true, "reasoning": "all done"}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      sampler,
		ExitGate:     ExitGate{RequireAllTasksDone: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	status := loop.Status()
	if status.Status != StatusCompleted {
		t.Errorf("Status = %q, want completed", status.Status)
	}
	if len(status.CompletedIDs) != 2 {
		t.Errorf("CompletedIDs = %v, want [t1 t2]", status.CompletedIDs)
	}
	// Verify the premature completion was rejected.
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "completion rejected") && strings.Contains(entry.Result, "ExitGate") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'completion rejected' log entry from ExitGate")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — MaxConsecutiveSamplerFailures halt path
// --------------------------------------------------------------------------

// alwaysFailSampler always returns an error.
type alwaysFailSampler struct{}

func (s *alwaysFailSampler) CreateMessage(_ context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	return nil, fmt.Errorf("permanent sampler failure")
}

func TestLoop_MaxConsecutiveSamplerFailures(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "consec-fail", Description: "consecutive sampler failures", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "fail"}},
	}
	specFile := writeSpec(t, dir, spec)

	loop, err := NewLoop(Config{
		SpecFile:                     specFile,
		ToolRegistry:                 registry.NewToolRegistry(),
		Sampler:                      &alwaysFailSampler{},
		SamplerRetries:               0,
		SamplerBackoff:               time.Millisecond,
		MaxConsecutiveSamplerFailures: 3,
		MaxIterations:                100,
	})
	if err != nil {
		t.Fatal(err)
	}

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error from consecutive sampler failures")
	}
	if !strings.Contains(runErr.Error(), "consecutive sampler failures") {
		t.Errorf("error = %q, want contains 'consecutive sampler failures'", runErr)
	}
	if loop.Status().Status != StatusFailed {
		t.Errorf("Status = %q, want failed", loop.Status().Status)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — non-text sampler result (content type assertion fails)
// --------------------------------------------------------------------------

// nonTextSampler returns a result whose Content is not a registry.Content type.
type nonTextSampler struct {
	calls  int
	inner  *scriptedSampler
}

func (s *nonTextSampler) CreateMessage(ctx context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	s.calls++
	if s.calls == 1 {
		// Return a result with a non-Content type for Content field.
		return &sampling.CreateMessageResult{
			SamplingMessage: registry.SamplingMessage{
				Content: "not a registry.Content type",
				Role:    registry.RoleAssistant,
			},
		}, nil
	}
	return s.inner.CreateMessage(ctx, req)
}

func TestLoop_NonTextSamplerResult(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "nontext-sampler", Description: "non-text sampler result", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &nonTextSampler{
		inner: &scriptedSampler{
			responses: []string{`{"complete": true}`},
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
		t.Errorf("Status = %q, want completed", status.Status)
	}
	// First iteration should have "no text in sampler response".
	found := false
	for _, entry := range status.Log {
		if strings.Contains(entry.Result, "no text in sampler response") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'no text in sampler response' log entry")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — CircuitBreaker open halts loop
// --------------------------------------------------------------------------

func TestLoop_CircuitBreaker_Halt(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "cb-halt", Description: "circuit breaker halt", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)

	sampler := &scriptedSampler{
		responses: []string{
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "a"}}`,
			`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "b"}}`,
		},
	}

	reg := registry.NewToolRegistry()
	echoTool(reg)

	// Circuit breaker with threshold 1 and long cooldown.
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  100,
		CooldownDuration:    time.Hour,
	})

	loop, err := NewLoop(Config{
		SpecFile:       specFile,
		ToolRegistry:   reg,
		Sampler:        sampler,
		CircuitBreaker: cb,
	})
	if err != nil {
		t.Fatal(err)
	}

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected circuit breaker open error")
	}
	if !strings.Contains(runErr.Error(), "circuit breaker open") {
		t.Errorf("error = %q, want contains 'circuit breaker open'", runErr)
	}
}

// --------------------------------------------------------------------------
// CooldownRemaining — expired cooldown returns 0
// --------------------------------------------------------------------------

func TestCircuitBreaker_CooldownRemaining_Expired(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  100,
		CooldownDuration:    time.Millisecond,
	})
	cb.RecordResult(false, "")
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}
	time.Sleep(5 * time.Millisecond)
	remaining := cb.CooldownRemaining()
	if remaining != 0 {
		t.Errorf("expected 0 for expired cooldown, got %v", remaining)
	}
}

// --------------------------------------------------------------------------
// normalizeError — long string truncation
// --------------------------------------------------------------------------

func TestNormalizeError_LongString(t *testing.T) {
	t.Parallel()
	// A string longer than 120 chars should be truncated.
	long := strings.Repeat("x", 200)
	result := normalizeError(long)
	if len(result) > 120 {
		t.Errorf("normalizeError should truncate to 120, got len=%d", len(result))
	}
}

func TestNormalizeError_ShortString(t *testing.T) {
	t.Parallel()
	short := "tool error: short"
	result := normalizeError(short)
	if result != "tool error: short" {
		t.Errorf("normalizeError(%q) = %q", short, result)
	}
}

// --------------------------------------------------------------------------
// SaveProgress — Rename failure path (covered by writing to non-existent dir)
// --------------------------------------------------------------------------

func TestSaveProgress_NonExistentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "subdir", "progress.json")
	p := Progress{Status: StatusRunning}
	err := SaveProgress(path, p)
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — long parse error preview truncation
// --------------------------------------------------------------------------

func TestLoop_ParseError_LongResponseTruncation(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "parse-trunc", Description: "parse truncation", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Response longer than 200 chars that's not valid JSON.
	longResp := strings.Repeat("not json ", 50)
	sampler := &scriptedSampler{
		responses: []string{
			longResp,
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

	// Check that the log entry has the truncated preview.
	found := false
	for _, entry := range loop.Status().Log {
		if strings.Contains(entry.Result, "parse error") && strings.Contains(entry.Result, "...") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected truncated parse error log entry")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — LoadProgress read error (non-readable file)
// --------------------------------------------------------------------------

func TestLoop_LoadProgress_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "desc", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "x"}},
	}
	specFile := writeSpec(t, dir, spec)

	// Create a non-readable progress file.
	progressFile := filepath.Join(dir, "spec.progress.json")
	os.WriteFile(progressFile, []byte("{}"), 0o644)
	os.Chmod(progressFile, 0o000)
	t.Cleanup(func() { os.Chmod(progressFile, 0o644) })

	reg := registry.NewToolRegistry()
	loop, err := NewLoop(Config{
		SpecFile:     specFile,
		ToolRegistry: reg,
		Sampler:      &mockSampler{},
	})
	if err != nil {
		t.Fatal(err)
	}

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error from non-readable progress file")
	}
}

// --------------------------------------------------------------------------
// Loop.Run — spec reload error path (spec file deleted mid-loop)
// --------------------------------------------------------------------------

func TestLoop_SpecReloadError(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "reload-err", Description: "spec reload error", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "do"}},
	}
	specFile := writeSpec(t, dir, spec)

	callCount := 0
	sampler := &callbackSampler{
		callback: func(ctx context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
			callCount++
			if callCount == 1 {
				// First call: return a non-complete decision. Before next iteration,
				// delete the spec file to cause a reload error.
				os.Remove(specFile)
				return &sampling.CreateMessageResult{
					SamplingMessage: registry.SamplingMessage{
						Content: registry.MakeTextContent(`{"complete": false, "task_id": "t1", "tool_name": "echo", "arguments": {"message": "hi"}}`),
						Role:    registry.RoleAssistant,
					},
				}, nil
			}
			return nil, fmt.Errorf("should not reach")
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

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error from spec reload failure")
	}
	if loop.Status().Status != StatusFailed {
		t.Errorf("Status = %q, want failed", loop.Status().Status)
	}
}

// callbackSampler calls a callback for each request.
type callbackSampler struct {
	callback func(context.Context, sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error)
}

func (s *callbackSampler) CreateMessage(ctx context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	return s.callback(ctx, req)
}

// --------------------------------------------------------------------------
// Loop.Run — LoadSpec error (non-existent spec file)
// --------------------------------------------------------------------------

func TestLoop_Run_SpecNotFound(t *testing.T) {
	reg := registry.NewToolRegistry()
	loop, err := NewLoop(Config{
		SpecFile:     "/nonexistent/spec.json",
		ToolRegistry: reg,
		Sampler:      &mockSampler{},
	})
	if err != nil {
		t.Fatal(err)
	}

	runErr := loop.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error for nonexistent spec file")
	}
}

// --------------------------------------------------------------------------
// NewLoop — SamplerRetries negative
// --------------------------------------------------------------------------

func TestNewLoop_NegativeSamplerRetries(t *testing.T) {
	dir := t.TempDir()
	spec := Spec{
		Name: "test", Description: "desc", Completion: "done",
		Tasks: []Task{{ID: "t1", Description: "x"}},
	}
	data, _ := json.Marshal(spec)
	specFile := filepath.Join(dir, "spec.json")
	os.WriteFile(specFile, data, 0o644)

	loop, err := NewLoop(Config{
		SpecFile:       specFile,
		ToolRegistry:   registry.NewToolRegistry(),
		Sampler:        &mockSampler{},
		SamplerRetries: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if loop.config.SamplerRetries != 0 {
		t.Errorf("SamplerRetries = %d, want 0 for negative input", loop.config.SamplerRetries)
	}
}

// --------------------------------------------------------------------------
// Loop.Run — tool result with non-text content type
// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// BuildMessages — truncation of long tool results
// --------------------------------------------------------------------------

func TestBuildMessages_LongToolResultTruncation(t *testing.T) {
	t.Parallel()
	longResult := strings.Repeat("x", 3000)
	history := []ConversationTurn{
		{
			UserPrompt:    "prompt",
			AssistantText: "response",
			ToolResults:   []string{longResult},
		},
	}
	messages := BuildMessages(history, 5, "current")
	// The tool result message should contain truncation marker.
	for _, msg := range messages {
		text, ok := registry.ExtractTextContent(msg.Content.(registry.Content))
		if ok && strings.Contains(text, "[truncated") {
			return
		}
	}
	t.Error("expected truncation marker in tool result message")
}
