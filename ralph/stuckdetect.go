package ralph

import (
	"fmt"
	"strings"
)

// StuckPattern identifies the type of stuck loop detected.
type StuckPattern string

const (
	// StuckRepeatedTool means the same tool+args were called N times.
	StuckRepeatedTool StuckPattern = "repeated_tool"
	// StuckRepeatedError means the same error occurred N times.
	StuckRepeatedError StuckPattern = "repeated_error"
	// StuckNoProgress means the same task was worked on without completion.
	StuckNoProgress StuckPattern = "no_progress"
)

// StuckSignal describes a detected stuck-loop condition.
type StuckSignal struct {
	Pattern    StuckPattern
	Detail     string // human-readable detail (e.g., which tool, which error)
	Suggestion string // corrective hint for the LLM
}

// StuckDetector analyzes recent iteration logs for stuck patterns.
type StuckDetector struct {
	Threshold int // minimum repeated occurrences to trigger (default 3)
}

// NewStuckDetector creates a detector with the given threshold.
func NewStuckDetector(threshold int) *StuckDetector {
	if threshold <= 0 {
		threshold = 3
	}
	return &StuckDetector{Threshold: threshold}
}

// Check examines the last N log entries for stuck patterns.
// Returns nil if no stuck pattern is detected.
func (d *StuckDetector) Check(log []IterationLog) *StuckSignal {
	if len(log) < d.Threshold {
		return nil
	}

	tail := log
	if len(tail) > d.Threshold*2 {
		tail = tail[len(tail)-d.Threshold*2:]
	}

	// Pattern 1: same tool+args repeated threshold times.
	if sig := d.checkRepeatedTool(tail); sig != nil {
		return sig
	}

	// Pattern 2: same error repeated threshold times.
	if sig := d.checkRepeatedError(tail); sig != nil {
		return sig
	}

	// Pattern 3: same task targeted for 5+ iterations with no mark_done.
	if sig := d.checkNoProgress(tail); sig != nil {
		return sig
	}

	return nil
}

func (d *StuckDetector) checkRepeatedTool(tail []IterationLog) *StuckSignal {
	if len(tail) < d.Threshold {
		return nil
	}
	// Check the last `threshold` entries for identical tool call lists.
	recent := tail[len(tail)-d.Threshold:]
	first := toolKey(recent[0])
	if first == "" {
		return nil
	}
	for _, entry := range recent[1:] {
		if toolKey(entry) != first {
			return nil
		}
	}
	return &StuckSignal{
		Pattern: StuckRepeatedTool,
		Detail:  fmt.Sprintf("tool call %q repeated %d times", first, d.Threshold),
		Suggestion: fmt.Sprintf(
			"You have called %q with the same arguments %d times in a row. "+
				"Try a different tool, different arguments, or re-read the error output to understand what went wrong.",
			first, d.Threshold),
	}
}

func (d *StuckDetector) checkRepeatedError(tail []IterationLog) *StuckSignal {
	if len(tail) < d.Threshold {
		return nil
	}
	recent := tail[len(tail)-d.Threshold:]
	// Only consider error results.
	firstErr := ""
	for _, entry := range recent {
		if !isErrorResult(entry.Result) {
			return nil
		}
		normalized := normalizeError(entry.Result)
		if firstErr == "" {
			firstErr = normalized
		} else if normalized != firstErr {
			return nil
		}
	}
	if firstErr == "" {
		return nil
	}
	return &StuckSignal{
		Pattern: StuckRepeatedError,
		Detail:  fmt.Sprintf("error %q repeated %d times", truncateStr(firstErr, 80), d.Threshold),
		Suggestion: fmt.Sprintf(
			"The same error has occurred %d times in a row. "+
				"Your current approach is not working. Try: (1) read the target file first, "+
				"(2) use a completely different strategy, (3) skip this task and try another.",
			d.Threshold),
	}
}

func (d *StuckDetector) checkNoProgress(tail []IterationLog) *StuckSignal {
	noProgressThreshold := d.Threshold + 2 // default 5 for threshold=3
	if len(tail) < noProgressThreshold {
		return nil
	}
	recent := tail[len(tail)-noProgressThreshold:]
	firstTask := recent[0].TaskID
	if firstTask == "" {
		return nil
	}
	for _, entry := range recent[1:] {
		if entry.TaskID != firstTask {
			return nil
		}
	}
	return &StuckSignal{
		Pattern: StuckNoProgress,
		Detail:  fmt.Sprintf("task %q worked on for %d iterations without completion", firstTask, noProgressThreshold),
		Suggestion: fmt.Sprintf(
			"Task %q has been in progress for %d iterations without being marked done. "+
				"Either: (1) the task is complete — set mark_done=true, "+
				"(2) you're blocked — try a different approach, "+
				"(3) the task is too large — break it into smaller steps.",
			firstTask, noProgressThreshold),
	}
}

// toolKey creates a string key from tool calls for comparison.
func toolKey(entry IterationLog) string {
	if len(entry.ToolCalls) == 0 {
		return ""
	}
	return strings.Join(entry.ToolCalls, "+")
}

// normalizeError strips variable parts from error messages for comparison.
func normalizeError(result string) string {
	// Trim to first 120 chars to ignore changing details at the end.
	s := strings.TrimSpace(result)
	if len(s) > 120 {
		s = s[:120]
	}
	return strings.ToLower(s)
}

// truncateStr truncates a string to maxLen with "..." suffix.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimRight(s[:maxLen-3], " ") + "..."
}
