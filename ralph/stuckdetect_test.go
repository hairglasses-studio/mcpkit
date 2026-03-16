package ralph

import (
	"testing"
)

func TestStuckDetector_NoStuck_InsufficientLog(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, ToolCalls: []string{"echo"}, Result: "ok"},
		{Iteration: 2, ToolCalls: []string{"echo"}, Result: "ok"},
	}
	if sig := d.Check(log); sig != nil {
		t.Errorf("expected nil for insufficient log, got %+v", sig)
	}
}

func TestStuckDetector_RepeatedTool(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, ToolCalls: []string{"write_file"}, Result: "ok"},
		{Iteration: 2, ToolCalls: []string{"write_file"}, Result: "ok"},
		{Iteration: 3, ToolCalls: []string{"write_file"}, Result: "ok"},
	}
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected stuck signal for repeated tool")
	}
	if sig.Pattern != StuckRepeatedTool {
		t.Errorf("pattern = %q, want %q", sig.Pattern, StuckRepeatedTool)
	}
}

func TestStuckDetector_RepeatedTool_DifferentTools(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, ToolCalls: []string{"write_file"}, Result: "ok"},
		{Iteration: 2, ToolCalls: []string{"read_file"}, Result: "ok"},
		{Iteration: 3, ToolCalls: []string{"write_file"}, Result: "ok"},
	}
	sig := d.Check(log)
	if sig != nil {
		t.Errorf("expected nil for different tools, got %+v", sig)
	}
}

func TestStuckDetector_RepeatedError(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, Result: "tool error: connection refused"},
		{Iteration: 2, Result: "tool error: connection refused"},
		{Iteration: 3, Result: "tool error: connection refused"},
	}
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected stuck signal for repeated error")
	}
	if sig.Pattern != StuckRepeatedError {
		t.Errorf("pattern = %q, want %q", sig.Pattern, StuckRepeatedError)
	}
}

func TestStuckDetector_RepeatedError_DifferentErrors(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, Result: "tool error: timeout"},
		{Iteration: 2, Result: "tool error: not found"},
		{Iteration: 3, Result: "tool error: timeout"},
	}
	sig := d.Check(log)
	if sig != nil {
		t.Errorf("expected nil for different errors, got %+v", sig)
	}
}

func TestStuckDetector_NoProgress(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3) // noProgressThreshold = 5
	log := []IterationLog{
		{Iteration: 1, TaskID: "implement", ToolCalls: []string{"write_file"}, Result: "ok"},
		{Iteration: 2, TaskID: "implement", ToolCalls: []string{"read_file"}, Result: "ok"},
		{Iteration: 3, TaskID: "implement", ToolCalls: []string{"write_file"}, Result: "ok"},
		{Iteration: 4, TaskID: "implement", ToolCalls: []string{"list_dir"}, Result: "ok"},
		{Iteration: 5, TaskID: "implement", ToolCalls: []string{"write_file"}, Result: "ok"},
	}
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected stuck signal for no progress")
	}
	if sig.Pattern != StuckNoProgress {
		t.Errorf("pattern = %q, want %q", sig.Pattern, StuckNoProgress)
	}
}

func TestStuckDetector_NoProgress_DifferentTasks(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, TaskID: "scan", Result: "ok"},
		{Iteration: 2, TaskID: "plan", Result: "ok"},
		{Iteration: 3, TaskID: "implement", Result: "ok"},
		{Iteration: 4, TaskID: "verify", Result: "ok"},
		{Iteration: 5, TaskID: "report", Result: "ok"},
	}
	if sig := d.Check(log); sig != nil {
		t.Errorf("expected nil for different tasks, got %+v", sig)
	}
}

func TestStuckDetector_NoProgress_EmptyTaskID(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, Result: "ok"},
		{Iteration: 2, Result: "ok"},
		{Iteration: 3, Result: "ok"},
		{Iteration: 4, Result: "ok"},
		{Iteration: 5, Result: "ok"},
	}
	if sig := d.Check(log); sig != nil {
		t.Errorf("expected nil for empty task IDs, got %+v", sig)
	}
}

func TestStuckDetector_RepeatedToolPriority(t *testing.T) {
	t.Parallel()
	// When both repeated tool and repeated error could match,
	// repeated tool should take priority (checked first).
	d := NewStuckDetector(3)
	log := []IterationLog{
		{Iteration: 1, ToolCalls: []string{"write_file"}, Result: "tool error: permission denied"},
		{Iteration: 2, ToolCalls: []string{"write_file"}, Result: "tool error: permission denied"},
		{Iteration: 3, ToolCalls: []string{"write_file"}, Result: "tool error: permission denied"},
	}
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected stuck signal")
	}
	if sig.Pattern != StuckRepeatedTool {
		t.Errorf("expected repeated_tool priority, got %q", sig.Pattern)
	}
}

func TestStuckDetector_DefaultThreshold(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(0) // should default to 3
	if d.Threshold != 3 {
		t.Errorf("default threshold = %d, want 3", d.Threshold)
	}
}

func TestStuckDetector_CustomThreshold(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(5)
	// 4 repeated tools should not trigger with threshold 5.
	log := []IterationLog{
		{Iteration: 1, ToolCalls: []string{"echo"}, Result: "ok"},
		{Iteration: 2, ToolCalls: []string{"echo"}, Result: "ok"},
		{Iteration: 3, ToolCalls: []string{"echo"}, Result: "ok"},
		{Iteration: 4, ToolCalls: []string{"echo"}, Result: "ok"},
	}
	if sig := d.Check(log); sig != nil {
		t.Errorf("threshold 5 should not trigger on 4 repeats, got %+v", sig)
	}
	// Add 5th — should trigger.
	log = append(log, IterationLog{Iteration: 5, ToolCalls: []string{"echo"}, Result: "ok"})
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected trigger at threshold 5")
	}
}

func TestStuckDetector_LongLogTruncation(t *testing.T) {
	t.Parallel()
	d := NewStuckDetector(3)
	// Build a long log where only the last 3 are stuck.
	var log []IterationLog
	for i := 1; i <= 20; i++ {
		tool := "different_tool"
		if i > 17 {
			tool = "stuck_tool"
		}
		log = append(log, IterationLog{
			Iteration: i,
			ToolCalls: []string{tool},
			Result:    "ok",
		})
	}
	sig := d.Check(log)
	if sig == nil {
		t.Fatal("expected stuck signal for last 3 entries")
	}
	if sig.Pattern != StuckRepeatedTool {
		t.Errorf("pattern = %q, want repeated_tool", sig.Pattern)
	}
}

func TestTruncateStr(t *testing.T) {
	t.Parallel()
	if got := truncateStr("short", 10); got != "short" {
		t.Errorf("truncateStr('short', 10) = %q, want 'short'", got)
	}
	if got := truncateStr("a very long string here", 10); got != "a very..." {
		t.Errorf("truncateStr long = %q, want 'a very...'", got)
	}
}

func TestNormalizeError(t *testing.T) {
	t.Parallel()
	e1 := normalizeError("Tool Error: Connection Refused at 10:30:45")
	e2 := normalizeError("tool error: connection refused at 10:30:45")
	if e1 != e2 {
		t.Errorf("normalizeError should be case-insensitive: %q != %q", e1, e2)
	}
}
