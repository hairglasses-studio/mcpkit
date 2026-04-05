package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// StreamingConfig controls bidirectional streaming translation between MCP
// progress notifications and A2A SSE events.
type StreamingConfig struct {
	// Enabled activates streaming translation in the bridge.
	Enabled bool

	// ProgressToSSE enables MCP progress notifications -> A2A SSE events.
	ProgressToSSE bool

	// SSEToProgress enables A2A SSE events -> MCP progress notifications.
	SSEToProgress bool
}

// DefaultStreamingConfig returns a StreamingConfig with all streaming enabled.
func DefaultStreamingConfig() StreamingConfig {
	return StreamingConfig{
		Enabled:       true,
		ProgressToSSE: true,
		SSEToProgress: true,
	}
}

// progressMetadataKey is the metadata key used to store progress percentage
// in A2A TaskStatusUpdateEvent metadata.
const progressMetadataKey = "progress"

// ProgressToStatusEvent translates an MCP progress notification into an A2A
// TaskStatusUpdateEvent. The progress value (0.0 to 1.0) is encoded both in
// the status message text and in event metadata for programmatic access.
//
// The task state is always WORKING: 100% progress indicates near-completion
// but does not map to COMPLETED (which requires a separate explicit event
// after the final artifact is emitted).
func (t *Translator) ProgressToStatusEvent(
	taskInfo a2atypes.TaskInfo,
	progress float64,
	message string,
) a2atypes.TaskStatusUpdateEvent {
	// Clamp progress to [0.0, 1.0].
	progress = clampProgress(progress)

	// Build the status message text. Include both percentage and message
	// so human-readable SSE consumers get full context.
	var msgText string
	pct := int(math.Round(progress * 100))
	if message != "" {
		msgText = fmt.Sprintf("[%d%%] %s", pct, message)
	} else {
		msgText = fmt.Sprintf("[%d%%] in progress", pct)
	}

	statusMsg := a2atypes.NewMessageForTask(
		a2atypes.MessageRoleAgent, taskInfo,
		a2atypes.NewTextPart(msgText),
	)

	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: taskInfo.ContextID,
		TaskID:    taskInfo.TaskID,
		Status: a2atypes.TaskStatus{
			State:   a2atypes.TaskStateWorking,
			Message: statusMsg,
		},
		Metadata: map[string]any{
			progressMetadataKey: progress,
		},
	}

	return event
}

// StatusEventToProgress extracts an MCP-compatible progress value and message
// from an A2A TaskStatusUpdateEvent. It uses the following extraction strategy:
//
//  1. Check event metadata for a numeric "progress" key (set by ProgressToStatusEvent).
//  2. Fall back to parsing the "[N%]" prefix from the status message text.
//
// Returns the progress (0.0 to 1.0) and the message. If no progress can be
// extracted, returns 0.0 with whatever message text is available.
func (t *Translator) StatusEventToProgress(event a2atypes.TaskStatusUpdateEvent) (float64, string) {
	var progress float64
	var message string

	// Strategy 1: Extract from metadata (preferred — lossless round-trip).
	if event.Metadata != nil {
		if v, ok := event.Metadata[progressMetadataKey]; ok {
			switch p := v.(type) {
			case float64:
				progress = p
			case int:
				progress = float64(p)
			case json.Number:
				if f, err := p.Float64(); err == nil {
					progress = f
				}
			}
		}
	}

	// Extract the message from the status message parts.
	if event.Status.Message != nil && len(event.Status.Message.Parts) > 0 {
		rawText := event.Status.Message.Parts[0].Text()

		// Strategy 2: If no metadata progress, try parsing "[N%]" prefix.
		if progress == 0 && rawText != "" {
			parsed, remainder := parsePctPrefix(rawText)
			if parsed >= 0 {
				progress = parsed
				message = remainder
			} else {
				message = rawText
			}
		} else {
			// We have metadata progress; strip the "[N%] " prefix if present.
			_, remainder := parsePctPrefix(rawText)
			if remainder != "" {
				message = remainder
			} else {
				message = rawText
			}
		}
	}

	progress = clampProgress(progress)
	return progress, message
}

// StreamingProgressReporter implements registry.ProgressReporter by translating
// MCP progress reports into A2A TaskStatusUpdateEvent values sent through a
// yield function. This is used in the BridgeExecutor to emit streaming progress
// events during tool execution.
type StreamingProgressReporter struct {
	taskInfo   a2atypes.TaskInfo
	translator *Translator
	yield      func(a2atypes.Event, error) bool
}

// Verify interface compliance at compile time.
var _ registry.ProgressReporter = (*StreamingProgressReporter)(nil)

// NewStreamingProgressReporter creates a reporter that converts MCP progress
// calls into A2A SSE events via the yield function.
func NewStreamingProgressReporter(
	taskInfo a2atypes.TaskInfo,
	translator *Translator,
	yield func(a2atypes.Event, error) bool,
) *StreamingProgressReporter {
	return &StreamingProgressReporter{
		taskInfo:   taskInfo,
		translator: translator,
		yield:      yield,
	}
}

// Report implements registry.ProgressReporter. Each call emits a
// TaskStatusUpdateEvent with WORKING state and progress metadata.
func (r *StreamingProgressReporter) Report(ctx context.Context, progress float64, message string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	event := r.translator.ProgressToStatusEvent(r.taskInfo, progress, message)
	if !r.yield(&event, nil) {
		return context.Canceled
	}
	return nil
}

// clampProgress restricts a progress value to the [0.0, 1.0] range.
func clampProgress(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// parsePctPrefix tries to parse a "[N%] message" format string.
// Returns the progress as a fraction (0.0–1.0) and the remaining message.
// Returns (-1, "") if the format doesn't match.
func parsePctPrefix(s string) (float64, string) {
	if !strings.HasPrefix(s, "[") {
		return -1, ""
	}
	idx := strings.Index(s, "%]")
	if idx < 2 {
		return -1, ""
	}
	numStr := s[1:idx]
	pct, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return -1, ""
	}
	// Extract the message after "%] ".
	remainder := s[idx+2:]
	remainder = strings.TrimPrefix(remainder, " ")
	return pct / 100.0, remainder
}
