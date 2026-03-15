package ralph

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// makeToolDef is a helper to construct a registry.ToolDefinition for testing.
func makeToolDef(name, description string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        name,
			Description: description,
		},
	}
}

// minimalSpec returns a spec with a single ready task for basic tests.
func minimalSpec() Spec {
	return Spec{
		Name:        "test-spec",
		Description: "A test specification",
		Completion:  "All tasks complete",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
		},
	}
}

func TestBuildIterationPrompt_SpecHeader(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "# Task: test-spec") {
		t.Errorf("prompt missing spec name header; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "A test specification") {
		t.Errorf("prompt missing spec description; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "**Completion criteria:** All tasks complete") {
		t.Errorf("prompt missing completion criteria; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_ReadyTask(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "### Ready (work on these)") {
		t.Errorf("prompt missing Ready section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- [ ] `t1`: First task") {
		t.Errorf("prompt missing ready task entry; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_CompletedTask(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task"},
		},
	}
	progress := Progress{
		CompletedIDs: []string{"t1"},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "### Completed") {
		t.Errorf("prompt missing Completed section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- [x] `t1`: First task") {
		t.Errorf("prompt missing completed task [x] marker; got:\n%s", prompt)
	}
	// t2 should still be ready (no deps)
	if !strings.Contains(prompt, "- [ ] `t2`: Second task") {
		t.Errorf("prompt missing ready task t2; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_TaskDoneInSpec(t *testing.T) {
	t.Parallel()
	// Task marked Done=true in the spec itself (not via progress.CompletedIDs)
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task", Done: true},
			{ID: "t2", Description: "Second task"},
		},
	}
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "### Completed") {
		t.Errorf("prompt missing Completed section for spec-Done task; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- [x] `t1`: First task") {
		t.Errorf("prompt missing [x] for spec-Done task; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_BlockedTask(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task", DependsOn: []string{"t1"}},
		},
	}
	// No completed tasks — t2 is blocked by t1
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "### Blocked (dependencies not met)") {
		t.Errorf("prompt missing Blocked section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "blocked by: t1") {
		t.Errorf("prompt should identify t1 as blocker; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "`t2`: Second task") {
		t.Errorf("prompt missing blocked task t2 entry; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_BlockedNotShownWhenDepComplete(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task", DependsOn: []string{"t1"}},
		},
	}
	// t1 is complete — t2 should now appear in Ready, not Blocked
	progress := Progress{
		CompletedIDs: []string{"t1"},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if strings.Contains(prompt, "### Blocked") {
		t.Errorf("prompt should not have Blocked section when dep is complete; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- [ ] `t2`: Second task") {
		t.Errorf("prompt should show t2 as ready after dep complete; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_DependencyLabel_Done(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task", DependsOn: []string{"t1"}},
		},
	}
	// t1 complete — t2 is ready and should show dep label with [done]
	progress := Progress{
		CompletedIDs: []string{"t1"},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "t1 [done]") {
		t.Errorf("prompt should show completed dep label '[done]' on ready task; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_DependencyLabel_NotDone(t *testing.T) {
	t.Parallel()
	// When all deps are present but none are done, the label shows deps without [done].
	// This appears in the Ready section when tasks have some deps in-progress.
	// Scenario: t3 depends on t1 and t2; t1 is complete, t2 is not.
	// So t3 is blocked (not ready), but let's check a ready task with partial deps:
	// t2 depends on t1, t1 is complete → t2 is ready → dep label shows "t1 [done]"
	// To test a dep without [done] in the label for a READY task, we need a task
	// with multiple deps where some are done and some not — but that would be blocked.
	// So we test via the blocked section: the blockers list shows plain dep IDs.
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task"},
			{ID: "t3", Description: "Third task", DependsOn: []string{"t1", "t2"}},
		},
	}
	// t1 complete, t2 not — t3 is blocked by t2
	progress := Progress{
		CompletedIDs: []string{"t1"},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	// t3 in blocked section; only t2 is a blocker (t1 is done)
	if !strings.Contains(prompt, "blocked by: t2") {
		t.Errorf("prompt should show t2 as the only blocker; got:\n%s", prompt)
	}
	// t1 [done] should appear in dep label on the ready t3... wait, t3 is blocked.
	// Let's check that the blocked entry for t3 is present.
	if !strings.Contains(prompt, "`t3`: Third task") {
		t.Errorf("prompt missing blocked task t3; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_RecentActivity(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{
		Log: []IterationLog{
			{Iteration: 1, TaskID: "t1", ToolCalls: []string{"tool_a"}, Result: "success"},
		},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "## Recent Activity") {
		t.Errorf("prompt missing Recent Activity section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Iteration 1") {
		t.Errorf("prompt missing iteration number in activity; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "[t1]") {
		t.Errorf("prompt missing task ID in activity log; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "called tool_a") {
		t.Errorf("prompt missing tool call in activity log; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "success") {
		t.Errorf("prompt missing result in activity log; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_RecentActivityNoTaskID(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{
		Log: []IterationLog{
			{Iteration: 2, Result: "nothing done"},
		},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "Iteration 2") {
		t.Errorf("prompt missing iteration number; got:\n%s", prompt)
	}
	// No task ID — should not have bracket notation
	if strings.Contains(prompt, "Iteration 2 [") {
		t.Errorf("prompt should not show task ID bracket when TaskID is empty; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "nothing done") {
		t.Errorf("prompt missing result text; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_LogTruncation(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	// Build 8 log entries — only the last 5 should appear.
	var logs []IterationLog
	for i := 1; i <= 8; i++ {
		logs = append(logs, IterationLog{
			Iteration: i,
			Result:    "result",
		})
	}
	progress := Progress{Log: logs}
	prompt := buildIterationPrompt(spec, progress, nil)

	// Iterations 1-3 should NOT appear; 4-8 should appear.
	for _, hidden := range []int{1, 2, 3} {
		// Check that "Iteration N:" is absent (using the colon at end of entry format)
		needle := "Iteration " + itoa(hidden) + ":"
		if strings.Contains(prompt, needle) {
			t.Errorf("prompt should not contain hidden iteration %d (showing last 5 only); got:\n%s", hidden, prompt)
		}
	}
	for _, shown := range []int{4, 5, 6, 7, 8} {
		needle := "Iteration " + itoa(shown) + ":"
		if !strings.Contains(prompt, needle) {
			t.Errorf("prompt should contain iteration %d; got:\n%s", shown, prompt)
		}
	}
}

func TestBuildIterationPrompt_LogExactlyFive(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	var logs []IterationLog
	for i := 1; i <= 5; i++ {
		logs = append(logs, IterationLog{Iteration: i, Result: "ok"})
	}
	progress := Progress{Log: logs}
	prompt := buildIterationPrompt(spec, progress, nil)

	for _, shown := range []int{1, 2, 3, 4, 5} {
		needle := "Iteration " + itoa(shown) + ":"
		if !strings.Contains(prompt, needle) {
			t.Errorf("prompt should contain iteration %d when exactly 5 logs; got:\n%s", shown, prompt)
		}
	}
}

func TestBuildIterationPrompt_NoLogNoRecentActivity(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if strings.Contains(prompt, "## Recent Activity") {
		t.Errorf("prompt should not have Recent Activity section when log is empty; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_ToolListing(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	tools := []registry.ToolDefinition{
		makeToolDef("search", "Search the web for information"),
		makeToolDef("write_file", "Write content to a file"),
	}
	prompt := buildIterationPrompt(spec, progress, tools)

	if !strings.Contains(prompt, "## Available Tools") {
		t.Errorf("prompt missing Available Tools section; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "### search") {
		t.Errorf("prompt missing tool 'search' header; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Search the web for information") {
		t.Errorf("prompt missing search tool description; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "### write_file") {
		t.Errorf("prompt missing tool 'write_file' header; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Write content to a file") {
		t.Errorf("prompt missing write_file tool description; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_NoTools(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	// Section header should still appear even with no tools.
	if !strings.Contains(prompt, "## Available Tools") {
		t.Errorf("prompt missing Available Tools section even with empty tool list; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_JSONDecisionPrompt(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "Respond with a JSON decision") {
		t.Errorf("prompt missing JSON decision instruction; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_MultipleToolCallsInLog(t *testing.T) {
	t.Parallel()
	spec := minimalSpec()
	progress := Progress{
		Log: []IterationLog{
			{
				Iteration: 1,
				TaskID:    "t1",
				ToolCalls: []string{"tool_a", "tool_b", "tool_c"},
				Result:    "multi-tool result",
			},
		},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "called tool_a, tool_b, tool_c") {
		t.Errorf("prompt should list multiple tool calls comma-separated; got:\n%s", prompt)
	}
}

func TestBuildIterationPrompt_AllSectionsPresent(t *testing.T) {
	t.Parallel()
	// A spec with all section types: ready, blocked, completed.
	spec := Spec{
		Name:        "full-spec",
		Description: "Tests all sections",
		Completion:  "Everything done",
		Tasks: []Task{
			{ID: "t1", Description: "Already complete"},
			{ID: "t2", Description: "Currently ready"},
			{ID: "t3", Description: "Depends on t2", DependsOn: []string{"t2"}},
		},
	}
	progress := Progress{
		CompletedIDs: []string{"t1"},
		Log: []IterationLog{
			{Iteration: 1, TaskID: "t1", ToolCalls: []string{"do_thing"}, Result: "done"},
		},
	}
	tools := []registry.ToolDefinition{makeToolDef("my_tool", "A useful tool")}
	prompt := buildIterationPrompt(spec, progress, tools)

	sections := []string{
		"### Ready (work on these)",
		"### Blocked (dependencies not met)",
		"### Completed",
		"## Recent Activity",
		"## Available Tools",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing section %q; got:\n%s", section, prompt)
		}
	}
}

func TestBuildIterationPrompt_DepLabelShowsDoneAndPending(t *testing.T) {
	t.Parallel()
	// t3 depends on both t1 (done) and t2 (done) → t3 is ready.
	// The dep label on t3 should say "t1 [done], t2 [done]".
	spec := Spec{
		Name:        "test",
		Description: "desc",
		Completion:  "done",
		Tasks: []Task{
			{ID: "t1", Description: "First task"},
			{ID: "t2", Description: "Second task"},
			{ID: "t3", Description: "Third task", DependsOn: []string{"t1", "t2"}},
		},
	}
	progress := Progress{
		CompletedIDs: []string{"t1", "t2"},
	}
	prompt := buildIterationPrompt(spec, progress, nil)

	if !strings.Contains(prompt, "t1 [done]") {
		t.Errorf("prompt should show t1 [done] in dep label; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "t2 [done]") {
		t.Errorf("prompt should show t2 [done] in dep label; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- [ ] `t3`: Third task") {
		t.Errorf("prompt should show t3 as ready; got:\n%s", prompt)
	}
}

// itoa is a minimal int-to-string helper for test assertions.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
