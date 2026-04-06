//go:build !official_sdk

package ralph

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/hitools"
	"github.com/hairglasses-studio/mcpkit/middleware/prefetch"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resilience"
	"github.com/hairglasses-studio/mcpkit/session"
)

// FactorConfig holds the optional 12-factor agent integration settings.
// All fields are optional; when nil/zero the loop runs with legacy behaviour.
type FactorConfig struct {
	// Thread is the session thread for append-only event logging (Factor 5).
	// When set, the loop appends EventToolCall, EventToolResult, and EventError
	// events for every tool invocation.
	Thread *session.Thread

	// CheckpointMgr enables pause/resume via the session checkpoint API (Factor 6).
	// When set, the loop checks for pause requests between iterations and saves
	// checkpoint state before pausing.
	CheckpointMgr *session.CheckpointManager

	// PauseRequested is polled at the start of each iteration. When it returns
	// true the loop pauses via CheckpointMgr.Pause and returns ErrLoopPaused.
	// Ignored when CheckpointMgr is nil.
	PauseRequested func() bool

	// ErrorFormatter formats tool errors for LLM context (Factor 9).
	// Defaults to resilience.CompactError when set to nil.
	ErrorFormatter func(error) string

	// ApprovalConfig configures human-in-the-loop approval (Factor 7).
	// When non-nil, tool calls matching ShouldApprove are gated on human
	// approval before execution. The approval flow runs inline within the
	// loop's tool execution path.
	ApprovalConfig *hitools.ApprovalMiddlewareConfig

	// PrefetchProviders registers pre-fetch data providers (Factor 13).
	// Before each iteration's tool execution, applicable providers are run
	// concurrently and their results are injected into the tool's context
	// via context.WithValue using prefetch.PrefetchFromContext for retrieval.
	PrefetchProviders map[string]prefetch.PrefetchProvider

	// PrefetchCacheTTL overrides the default cache TTL for prefetch providers.
	// Zero means use prefetch.DefaultCacheTTL.
	PrefetchCacheTTL time.Duration
}

// ErrLoopPaused is returned by Run when the loop is paused via a checkpoint.
// The caller can later resume by calling Run again after restoring the thread.
var ErrLoopPaused = fmt.Errorf("ralph: loop paused at checkpoint")

// threadToolCallData is the JSON-serializable payload for tool_call events.
type threadToolCallData struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Iteration int            `json:"iteration"`
}

// threadToolResultData is the JSON-serializable payload for tool_result events.
type threadToolResultData struct {
	ToolName  string `json:"tool_name"`
	Result    string `json:"result"`
	IsError   bool   `json:"is_error,omitempty"`
	Iteration int    `json:"iteration"`
}

// threadErrorData is the JSON-serializable payload for error events.
type threadErrorData struct {
	ToolName  string `json:"tool_name,omitempty"`
	Error     string `json:"error"`
	Formatted string `json:"formatted,omitempty"`
	Iteration int    `json:"iteration"`
}

// appendToolCallEvent appends an EventToolCall to the thread if configured.
func (l *Loop) appendToolCallEvent(call ToolCall, taskID string, iteration int) {
	fc := l.config.FactorConfig
	if fc == nil || fc.Thread == nil {
		return
	}
	data := threadToolCallData{
		ToolName:  call.Name,
		Arguments: call.Arguments,
		TaskID:    taskID,
		Iteration: iteration,
	}
	evt, err := session.NewEvent(session.EventToolCall, data)
	if err != nil {
		return // best-effort; do not break the loop
	}
	evt.Metadata["iteration"] = fmt.Sprintf("%d", iteration)
	if taskID != "" {
		evt.Metadata["task_id"] = taskID
	}
	fc.Thread.Append(evt)
}

// appendToolResultEvent appends an EventToolResult to the thread if configured.
func (l *Loop) appendToolResultEvent(toolName, result string, isError bool, iteration int) {
	fc := l.config.FactorConfig
	if fc == nil || fc.Thread == nil {
		return
	}
	data := threadToolResultData{
		ToolName:  toolName,
		Result:    truncateForThread(result, 2048),
		IsError:   isError,
		Iteration: iteration,
	}
	evt, err := session.NewEvent(session.EventToolResult, data)
	if err != nil {
		return
	}
	evt.Metadata["iteration"] = fmt.Sprintf("%d", iteration)
	fc.Thread.Append(evt)
}

// appendErrorEvent appends an EventError to the thread if configured, using
// the error formatter (Factor 9) to produce a compact LLM-friendly message.
func (l *Loop) appendErrorEvent(toolName string, toolErr error, iteration int) {
	fc := l.config.FactorConfig
	if fc == nil || fc.Thread == nil {
		return
	}
	formatter := fc.ErrorFormatter
	if formatter == nil {
		formatter = resilience.CompactError
	}
	formatted := formatter(toolErr)
	data := threadErrorData{
		ToolName:  toolName,
		Error:     toolErr.Error(),
		Formatted: formatted,
		Iteration: iteration,
	}
	evt, err := session.NewEvent(session.EventError, data)
	if err != nil {
		return
	}
	evt.Metadata["iteration"] = fmt.Sprintf("%d", iteration)
	if toolName != "" {
		evt.Metadata["tool_name"] = toolName
	}
	fc.Thread.Append(evt)
}

// checkPauseRequested checks if a pause has been requested and, if so,
// pauses the loop via the checkpoint manager. Returns ErrLoopPaused when
// paused, nil otherwise.
func (l *Loop) checkPauseRequested(ctx context.Context) error {
	fc := l.config.FactorConfig
	if fc == nil || fc.CheckpointMgr == nil || fc.PauseRequested == nil {
		return nil
	}
	if !fc.PauseRequested() {
		return nil
	}
	if fc.Thread == nil {
		return ErrLoopPaused
	}
	_, err := fc.CheckpointMgr.Pause(ctx, fc.Thread, "ralph loop paused by request")
	if err != nil {
		return fmt.Errorf("ralph: checkpoint pause failed: %w", err)
	}
	l.setStatus(StatusStopped)
	return ErrLoopPaused
}

// factorFormatError uses the 12-factor error formatter when configured,
// otherwise returns the raw error string.
func (l *Loop) factorFormatError(err error) string {
	fc := l.config.FactorConfig
	if fc == nil || fc.ErrorFormatter == nil {
		return err.Error()
	}
	return fc.ErrorFormatter(err)
}

// buildPrefetchContext creates a context enriched with pre-fetched data from
// Factor 13 providers. Falls back to the original context when no providers
// are configured. Uses the prefetch.Middleware internally to get proper caching
// and concurrent execution.
func (l *Loop) buildPrefetchContext(ctx context.Context, toolName string) context.Context {
	fc := l.config.FactorConfig
	if fc == nil || len(fc.PrefetchProviders) == 0 {
		return ctx
	}

	ttl := fc.PrefetchCacheTTL
	if ttl <= 0 {
		ttl = prefetch.DefaultCacheTTL
	}

	cfg := prefetch.Config{
		Providers:     fc.PrefetchProviders,
		CacheTTL:      ttl,
		MaxConcurrent: prefetch.DefaultMaxConcurrent,
	}

	// Use the prefetch middleware to get the enriched context. We construct a
	// passthrough handler that captures the context after prefetch injection.
	var enriched context.Context
	mw := prefetch.Middleware(cfg)

	dummyTD := registry.ToolDefinition{
		Tool: registry.Tool{Name: toolName},
	}
	passthrough := func(innerCtx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		enriched = innerCtx
		return nil, nil
	}
	handler := mw(toolName, dummyTD, passthrough)
	_, _ = handler(ctx, makeCallToolRequest(toolName, nil))

	if enriched != nil {
		return enriched
	}
	return ctx
}

// checkApproval checks whether a tool call requires human approval (Factor 7).
// Returns true to proceed, false (with an error message) to skip.
func (l *Loop) checkApproval(ctx context.Context, toolName string) (bool, string) {
	fc := l.config.FactorConfig
	if fc == nil || fc.ApprovalConfig == nil {
		return true, ""
	}
	ac := fc.ApprovalConfig
	if ac.ShouldApprove == nil {
		return true, ""
	}
	td, found := l.config.ToolRegistry.GetTool(toolName)
	if !found {
		return true, "" // let the tool-not-found path handle it
	}
	if !ac.ShouldApprove(toolName, td) {
		return true, "" // no approval needed for this tool
	}

	// Build and submit an approval request.
	id, err := generateFactorApprovalID()
	if err != nil {
		return false, fmt.Sprintf("[APPROVAL_ERROR] failed to generate ID: %v", err)
	}

	now := time.Now()
	req := hitools.ApprovalRequest{
		ID:        id,
		ToolName:  toolName,
		Action:    fmt.Sprintf("Execute tool %q in ralph loop", toolName),
		Urgency:   ac.DefaultUrgency,
		CreatedAt: now,
	}
	if req.Urgency == "" {
		req.Urgency = hitools.ApprovalUrgencyNormal
	}
	if ac.Timeout > 0 {
		req.ExpiresAt = now.Add(ac.Timeout)
	}

	if err := ac.Store.Submit(ctx, req); err != nil {
		return false, fmt.Sprintf("[APPROVAL_ERROR] failed to submit request: %v", err)
	}

	if ac.OnRequest != nil {
		ac.OnRequest(ctx, req)
	}

	// Wait for the response.
	resp, err := waitForFactorApproval(ctx, ac.Store, id, ac.Timeout)
	if err != nil {
		return false, fmt.Sprintf("[APPROVAL_TIMEOUT] %v", err)
	}

	if ac.OnResponse != nil {
		ac.OnResponse(ctx, req, *resp)
	}

	switch resp.Decision {
	case hitools.Approved, hitools.Modified:
		return true, ""
	case hitools.Denied:
		msg := "[APPROVAL_DENIED] tool call denied by human"
		if resp.Comment != "" {
			msg = fmt.Sprintf("[APPROVAL_DENIED] %s", resp.Comment)
		}
		return false, msg
	default:
		return false, fmt.Sprintf("[APPROVAL_ERROR] unknown decision: %q", resp.Decision)
	}
}

// factorCompleteThread marks the thread as completed via the checkpoint manager
// when 12-factor integration is configured. Called when the loop finishes
// successfully.
func (l *Loop) factorCompleteThread(ctx context.Context) {
	fc := l.config.FactorConfig
	if fc == nil || fc.CheckpointMgr == nil || fc.Thread == nil {
		return
	}
	_, _ = fc.CheckpointMgr.Complete(ctx, fc.Thread)
}

// factorFailThread marks the thread as failed via the checkpoint manager
// when 12-factor integration is configured. Called when the loop terminates
// with an error.
func (l *Loop) factorFailThread(ctx context.Context, reason string) {
	fc := l.config.FactorConfig
	if fc == nil || fc.CheckpointMgr == nil || fc.Thread == nil {
		return
	}
	_, _ = fc.CheckpointMgr.Fail(ctx, fc.Thread, reason)
}

// truncateForThread truncates a string to a maximum length for thread events,
// avoiding excessive memory usage in the event log.
func truncateForThread(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// waitForFactorApproval polls the approval store. This is a simplified version
// of the approval middleware's wait logic, suitable for the ralph loop context.
func waitForFactorApproval(ctx context.Context, store hitools.ApprovalStore, requestID string, timeout time.Duration) (*hitools.ApprovalResponse, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	interval := 50 * time.Millisecond
	maxInterval := 500 * time.Millisecond

	for {
		resp, err := store.GetResponse(ctx, requestID)
		if err != nil {
			return nil, fmt.Errorf("failed to check approval: %w", err)
		}
		if resp != nil {
			return resp, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("approval %q timed out or cancelled", requestID)
		case <-time.After(interval):
		}
		if interval < maxInterval {
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}

// prefetchCache holds cached prefetch results shared across iterations.
// This allows the Ralph loop to reuse prefetch results within a TTL window
// without reconstructing the middleware each call.
type prefetchCache struct {
	mu      sync.RWMutex
	entries map[string]prefetchCacheEntry
}

// prefetchCacheEntry holds a single cached prefetch result.
type prefetchCacheEntry struct {
	value     any
	expiresAt time.Time
}

// generateFactorApprovalID generates a random ID for factor approval requests.
func generateFactorApprovalID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ralph-apr-" + hex.EncodeToString(b), nil
}
