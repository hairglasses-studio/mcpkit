package a2a

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultTaskTimeout is the maximum duration for a single tool execution
// wrapped as an A2A task.
const DefaultTaskTimeout = 30 * time.Second

// BridgeExecutor implements a2asrv.AgentExecutor by translating A2A task
// messages into mcpkit tool calls. Each incoming A2A message is routed to the
// corresponding MCP tool handler via the registry, and the result is translated
// back into A2A events.
type BridgeExecutor struct {
	registry   *registry.ToolRegistry
	translator *Translator
	logger     *slog.Logger
	middleware []registry.Middleware
	timeout    time.Duration
}

// Verify interface compliance at compile time.
var _ a2asrv.AgentExecutor = (*BridgeExecutor)(nil)

// ExecutorConfig configures a BridgeExecutor.
type ExecutorConfig struct {
	// Translator overrides the default translator. If nil, a zero-value
	// Translator is used.
	Translator *Translator

	// Logger for executor operations. If nil, slog.Default() is used.
	Logger *slog.Logger

	// Middleware are mcpkit middleware applied to tool invocations through the
	// bridge. These run in addition to any middleware already configured on the
	// registry.
	Middleware []registry.Middleware

	// TaskTimeout is the maximum duration for a single tool execution.
	// Default: 30s.
	TaskTimeout time.Duration
}

// NewBridgeExecutor creates an executor bound to the given registry.
func NewBridgeExecutor(reg *registry.ToolRegistry, cfg ExecutorConfig) *BridgeExecutor {
	translator := cfg.Translator
	if translator == nil {
		translator = &Translator{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	timeout := cfg.TaskTimeout
	if timeout == 0 {
		timeout = DefaultTaskTimeout
	}
	return &BridgeExecutor{
		registry:   reg,
		translator: translator,
		logger:     logger,
		middleware: cfg.Middleware,
		timeout:    timeout,
	}
}

// Execute processes an A2A task by:
//  1. Extracting the target skill ID from the message
//  2. Looking up the corresponding MCP tool in the registry
//  3. Translating the A2A message parts to MCP CallToolRequest arguments
//  4. Invoking the tool handler through the middleware chain
//  5. Translating the CallToolResult to A2A events
//
// Returns an iter.Seq2[a2a.Event, error] as required by the AgentExecutor interface.
func (e *BridgeExecutor) Execute(
	ctx context.Context,
	execCtx *a2asrv.ExecutorContext,
) iter.Seq2[a2atypes.Event, error] {
	return func(yield func(a2atypes.Event, error) bool) {
		taskInfo := execCtx.TaskInfo()

		// 1. Emit submitted task if this is a new task.
		if execCtx.StoredTask == nil {
			submitted := a2atypes.NewSubmittedTask(execCtx, execCtx.Message)
			if !yield(submitted, nil) {
				return
			}
		}

		// 2. Extract skill ID and arguments from the incoming message.
		msg := execCtx.Message
		if msg == nil {
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart("no message in execution context"),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// Use an empty skill hint; the translator will extract skill ID from
		// the DataPart's "skill" field.
		skillID, args, err := e.translator.MessageToCallToolRequest(*msg, a2atypes.AgentSkill{})
		if err != nil {
			e.logger.Warn("failed to translate A2A message", "error", err)
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(fmt.Sprintf("invalid request: %v", err)),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// 3. Look up the MCP tool in the registry.
		td, ok := e.registry.GetTool(skillID)
		if !ok {
			e.logger.Warn("tool not found", "skill", skillID)
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(fmt.Sprintf("unknown tool: %s", skillID)),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// 4. Emit WORKING status.
		if !yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateWorking, nil), nil) {
			return
		}

		// 5. Build the MCP CallToolRequest.
		callReq := registry.CallToolRequest{}
		callReq.Params.Name = skillID
		callReq.Params.Arguments = args

		// 6. Apply bridge-level middleware chain and execute.
		handler := td.Handler
		for i := len(e.middleware) - 1; i >= 0; i-- {
			handler = e.middleware[i](skillID, td, handler)
		}

		// Execute with timeout.
		toolCtx, cancel := context.WithTimeout(ctx, e.timeout)
		defer cancel()

		result, toolErr := handler(toolCtx, callReq)

		e.logger.Info("tool executed",
			"skill", skillID,
			"error", toolErr,
			"is_error", registry.IsResultError(result),
		)

		// 7. Handle tool execution error.
		if toolErr != nil {
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(fmt.Sprintf("tool error: %v", toolErr)),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// 8. Handle error results from the tool (IsError flag).
		if result != nil && registry.IsResultError(result) {
			artifact := e.translator.CallResultToArtifact(result)
			errText := "tool returned an error"
			if len(artifact.Parts) > 0 {
				if t := artifact.Parts[0].Text(); t != "" {
					errText = t
				}
			}
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart(errText),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// 9. Handle nil result.
		if result == nil {
			errMsg := a2atypes.NewMessageForTask(
				a2atypes.MessageRoleAgent, taskInfo,
				a2atypes.NewTextPart("tool returned nil result"),
			)
			yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateFailed, errMsg), nil)
			return
		}

		// 10. Success: emit artifact with the result content.
		artifact := e.translator.CallResultToArtifact(result)
		artifactEvent := a2atypes.NewArtifactEvent(taskInfo, artifact.Parts...)
		if !yield(artifactEvent, nil) {
			return
		}

		// 11. Emit completed status.
		yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateCompleted, nil), nil)
	}
}

// Cancel handles task cancellation. Since MCP tool calls are synchronous
// and short-lived, cancellation emits a canceled status event.
func (e *BridgeExecutor) Cancel(
	ctx context.Context,
	execCtx *a2asrv.ExecutorContext,
) iter.Seq2[a2atypes.Event, error] {
	return func(yield func(a2atypes.Event, error) bool) {
		taskInfo := execCtx.TaskInfo()
		yield(a2atypes.NewStatusUpdateEvent(taskInfo, a2atypes.TaskStateCanceled, nil), nil)
	}
}
