package a2a

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
)

func streamingTaskInfo() a2atypes.TaskInfo {
	return a2atypes.TaskInfo{ContextID: "ctx-stream-1", TaskID: "task-stream-1"}
}

// --- ProgressToStatusEvent tests ---

func TestProgressToStatusEvent_BasicMapping(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()
	event := tr.ProgressToStatusEvent(info, 0.5, "Loading data")

	// State must be WORKING regardless of progress value.
	if event.Status.State != a2atypes.TaskStateWorking {
		t.Errorf("state = %q, want %q", event.Status.State, a2atypes.TaskStateWorking)
	}

	// Check task identity propagation.
	if event.TaskID != info.TaskID {
		t.Errorf("taskID = %q, want %q", event.TaskID, info.TaskID)
	}
	if event.ContextID != info.ContextID {
		t.Errorf("contextID = %q, want %q", event.ContextID, info.ContextID)
	}

	// Check metadata carries the progress value.
	if event.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
	p, ok := event.Metadata[progressMetadataKey].(float64)
	if !ok {
		t.Fatalf("metadata[progress] type = %T, want float64", event.Metadata[progressMetadataKey])
	}
	if p != 0.5 {
		t.Errorf("metadata[progress] = %v, want 0.5", p)
	}

	// Check the status message text includes percentage and message.
	if event.Status.Message == nil || len(event.Status.Message.Parts) == 0 {
		t.Fatal("expected non-empty status message")
	}
	text := event.Status.Message.Parts[0].Text()
	if text != "[50%] Loading data" {
		t.Errorf("message text = %q, want %q", text, "[50%] Loading data")
	}
}

func TestProgressToStatusEvent_ZeroProgress(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := tr.ProgressToStatusEvent(streamingTaskInfo(), 0.0, "Starting")

	if event.Status.State != a2atypes.TaskStateWorking {
		t.Errorf("state = %q, want %q", event.Status.State, a2atypes.TaskStateWorking)
	}

	p := event.Metadata[progressMetadataKey].(float64)
	if p != 0.0 {
		t.Errorf("metadata[progress] = %v, want 0.0", p)
	}

	text := event.Status.Message.Parts[0].Text()
	if text != "[0%] Starting" {
		t.Errorf("message text = %q, want %q", text, "[0%] Starting")
	}
}

func TestProgressToStatusEvent_FullProgress(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := tr.ProgressToStatusEvent(streamingTaskInfo(), 1.0, "Almost done")

	// 100% progress must still be WORKING, not COMPLETED.
	if event.Status.State != a2atypes.TaskStateWorking {
		t.Errorf("state = %q, want WORKING (not COMPLETED); 100%% maps to near-completion", event.Status.State)
	}

	p := event.Metadata[progressMetadataKey].(float64)
	if p != 1.0 {
		t.Errorf("metadata[progress] = %v, want 1.0", p)
	}

	text := event.Status.Message.Parts[0].Text()
	if text != "[100%] Almost done" {
		t.Errorf("message text = %q, want %q", text, "[100%] Almost done")
	}
}

func TestProgressToStatusEvent_EmptyMessage(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := tr.ProgressToStatusEvent(streamingTaskInfo(), 0.25, "")

	text := event.Status.Message.Parts[0].Text()
	if text != "[25%] in progress" {
		t.Errorf("message text = %q, want %q", text, "[25%] in progress")
	}
}

func TestProgressToStatusEvent_ClampNegative(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := tr.ProgressToStatusEvent(streamingTaskInfo(), -0.5, "underflow")

	p := event.Metadata[progressMetadataKey].(float64)
	if p != 0.0 {
		t.Errorf("metadata[progress] = %v, want 0.0 (clamped)", p)
	}
}

func TestProgressToStatusEvent_ClampOverflow(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := tr.ProgressToStatusEvent(streamingTaskInfo(), 1.5, "overflow")

	p := event.Metadata[progressMetadataKey].(float64)
	if p != 1.0 {
		t.Errorf("metadata[progress] = %v, want 1.0 (clamped)", p)
	}
}

// --- StatusEventToProgress tests ---

func TestStatusEventToProgress_FromMetadata(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: info.ContextID,
		TaskID:    info.TaskID,
		Status: a2atypes.TaskStatus{
			State: a2atypes.TaskStateWorking,
			Message: a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, info,
				a2atypes.NewTextPart("[75%] Processing records"),
			),
		},
		Metadata: map[string]any{
			progressMetadataKey: 0.75,
		},
	}

	progress, msg := tr.StatusEventToProgress(event)
	if progress != 0.75 {
		t.Errorf("progress = %v, want 0.75", progress)
	}
	if msg != "Processing records" {
		t.Errorf("message = %q, want %q", msg, "Processing records")
	}
}

func TestStatusEventToProgress_FromTextFallback(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	// No metadata -- progress extracted from text prefix.
	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: info.ContextID,
		TaskID:    info.TaskID,
		Status: a2atypes.TaskStatus{
			State: a2atypes.TaskStateWorking,
			Message: a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, info,
				a2atypes.NewTextPart("[30%] Indexing files"),
			),
		},
	}

	progress, msg := tr.StatusEventToProgress(event)
	if math.Abs(progress-0.30) > 0.001 {
		t.Errorf("progress = %v, want ~0.30", progress)
	}
	if msg != "Indexing files" {
		t.Errorf("message = %q, want %q", msg, "Indexing files")
	}
}

func TestStatusEventToProgress_NoProgressInfo(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	// Plain text message with no percentage format.
	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: info.ContextID,
		TaskID:    info.TaskID,
		Status: a2atypes.TaskStatus{
			State: a2atypes.TaskStateWorking,
			Message: a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, info,
				a2atypes.NewTextPart("doing stuff"),
			),
		},
	}

	progress, msg := tr.StatusEventToProgress(event)
	if progress != 0 {
		t.Errorf("progress = %v, want 0 (no progress info)", progress)
	}
	if msg != "doing stuff" {
		t.Errorf("message = %q, want %q", msg, "doing stuff")
	}
}

func TestStatusEventToProgress_NilMessage(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: "ctx",
		TaskID:    "task",
		Status: a2atypes.TaskStatus{
			State:   a2atypes.TaskStateWorking,
			Message: nil,
		},
	}

	progress, msg := tr.StatusEventToProgress(event)
	if progress != 0 {
		t.Errorf("progress = %v, want 0", progress)
	}
	if msg != "" {
		t.Errorf("message = %q, want empty", msg)
	}
}

func TestStatusEventToProgress_JSONNumberMetadata(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	// Simulate JSON decoding with UseNumber: metadata value is json.Number.
	event := a2atypes.TaskStatusUpdateEvent{
		ContextID: info.ContextID,
		TaskID:    info.TaskID,
		Status: a2atypes.TaskStatus{
			State: a2atypes.TaskStateWorking,
			Message: a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, info,
				a2atypes.NewTextPart("[60%] Halfway there"),
			),
		},
		Metadata: map[string]any{
			progressMetadataKey: json.Number("0.6"),
		},
	}

	progress, msg := tr.StatusEventToProgress(event)
	if math.Abs(progress-0.6) > 0.001 {
		t.Errorf("progress = %v, want ~0.6", progress)
	}
	if msg != "Halfway there" {
		t.Errorf("message = %q, want %q", msg, "Halfway there")
	}
}

// --- Round-trip tests ---

func TestRoundTrip_ProgressToEventToProgress(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	// Forward: MCP progress -> A2A event.
	event := tr.ProgressToStatusEvent(info, 0.73, "Scanning documents")

	// Reverse: A2A event -> MCP progress.
	progress, msg := tr.StatusEventToProgress(event)

	if math.Abs(progress-0.73) > 0.001 {
		t.Errorf("round-trip progress = %v, want ~0.73", progress)
	}
	if msg != "Scanning documents" {
		t.Errorf("round-trip message = %q, want %q", msg, "Scanning documents")
	}
}

func TestRoundTrip_ZeroProgress(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	event := tr.ProgressToStatusEvent(info, 0.0, "Initializing")
	progress, msg := tr.StatusEventToProgress(event)

	if progress != 0.0 {
		t.Errorf("round-trip progress = %v, want 0.0", progress)
	}
	if msg != "Initializing" {
		t.Errorf("round-trip message = %q, want %q", msg, "Initializing")
	}
}

func TestRoundTrip_FullProgress(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	event := tr.ProgressToStatusEvent(info, 1.0, "Finalizing")
	progress, msg := tr.StatusEventToProgress(event)

	if progress != 1.0 {
		t.Errorf("round-trip progress = %v, want 1.0", progress)
	}
	if msg != "Finalizing" {
		t.Errorf("round-trip message = %q, want %q", msg, "Finalizing")
	}
}

// --- StreamingProgressReporter tests ---

func TestStreamingProgressReporter_EmitsEvents(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	var events []a2atypes.Event
	yield := func(ev a2atypes.Event, err error) bool {
		events = append(events, ev)
		return true
	}

	reporter := NewStreamingProgressReporter(info, tr, yield)

	if err := reporter.Report(context.Background(), 0.25, "step 1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reporter.Report(context.Background(), 0.75, "step 2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify first event.
	ev1, ok := events[0].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("event[0] type = %T, want *TaskStatusUpdateEvent", events[0])
	}
	if ev1.Status.State != a2atypes.TaskStateWorking {
		t.Errorf("event[0] state = %q, want WORKING", ev1.Status.State)
	}
	p1 := ev1.Metadata[progressMetadataKey].(float64)
	if p1 != 0.25 {
		t.Errorf("event[0] progress = %v, want 0.25", p1)
	}

	// Verify second event.
	ev2, ok := events[1].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("event[1] type = %T, want *TaskStatusUpdateEvent", events[1])
	}
	p2 := ev2.Metadata[progressMetadataKey].(float64)
	if p2 != 0.75 {
		t.Errorf("event[1] progress = %v, want 0.75", p2)
	}
}

func TestStreamingProgressReporter_CanceledContext(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	yield := func(ev a2atypes.Event, err error) bool {
		t.Error("yield should not be called on canceled context")
		return true
	}

	reporter := NewStreamingProgressReporter(info, tr, yield)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := reporter.Report(ctx, 0.5, "should not emit")
	if err == nil {
		t.Error("expected error from canceled context")
	}
}

func TestStreamingProgressReporter_YieldReturnsFalse(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	info := streamingTaskInfo()

	callCount := 0
	yield := func(ev a2atypes.Event, err error) bool {
		callCount++
		return false // Consumer stopped listening.
	}

	reporter := NewStreamingProgressReporter(info, tr, yield)

	err := reporter.Report(context.Background(), 0.5, "first")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected yield called once, got %d", callCount)
	}
}

// --- StreamingConfig tests ---

func TestDefaultStreamingConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultStreamingConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if !cfg.ProgressToSSE {
		t.Error("expected ProgressToSSE = true")
	}
	if !cfg.SSEToProgress {
		t.Error("expected SSEToProgress = true")
	}
}

// --- clampProgress tests ---

func TestClampProgress_InRange(t *testing.T) {
	t.Parallel()
	if v := clampProgress(0.5); v != 0.5 {
		t.Errorf("clampProgress(0.5) = %v, want 0.5", v)
	}
}

func TestClampProgress_Negative(t *testing.T) {
	t.Parallel()
	if v := clampProgress(-1); v != 0 {
		t.Errorf("clampProgress(-1) = %v, want 0", v)
	}
}

func TestClampProgress_Over(t *testing.T) {
	t.Parallel()
	if v := clampProgress(2); v != 1 {
		t.Errorf("clampProgress(2) = %v, want 1", v)
	}
}

// --- parsePctPrefix tests ---

func TestParsePctPrefix_Valid(t *testing.T) {
	t.Parallel()
	pct, msg := parsePctPrefix("[50%] Processing")
	if math.Abs(pct-0.5) > 0.001 {
		t.Errorf("pct = %v, want ~0.5", pct)
	}
	if msg != "Processing" {
		t.Errorf("msg = %q, want %q", msg, "Processing")
	}
}

func TestParsePctPrefix_NoPrefix(t *testing.T) {
	t.Parallel()
	pct, msg := parsePctPrefix("no prefix here")
	if pct != -1 {
		t.Errorf("pct = %v, want -1", pct)
	}
	if msg != "" {
		t.Errorf("msg = %q, want empty", msg)
	}
}

func TestParsePctPrefix_ZeroPct(t *testing.T) {
	t.Parallel()
	pct, msg := parsePctPrefix("[0%] Starting")
	if pct != 0 {
		t.Errorf("pct = %v, want 0", pct)
	}
	if msg != "Starting" {
		t.Errorf("msg = %q, want %q", msg, "Starting")
	}
}

func TestParsePctPrefix_100Pct(t *testing.T) {
	t.Parallel()
	pct, msg := parsePctPrefix("[100%] Done")
	if pct != 1.0 {
		t.Errorf("pct = %v, want 1.0", pct)
	}
	if msg != "Done" {
		t.Errorf("msg = %q, want %q", msg, "Done")
	}
}
