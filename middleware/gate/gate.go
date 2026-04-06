//go:build !official_sdk

package gate

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Verdict is the decision returned by a GateFunc.
type Verdict int

const (
	// VerdictProceed allows the tool call to execute immediately.
	VerdictProceed Verdict = iota
	// VerdictPause blocks execution until the ApprovalFunc resolves.
	// Use this for human-in-the-loop confirmation flows.
	VerdictPause
	// VerdictDeny rejects the tool call and returns an error result.
	VerdictDeny
)

func (v Verdict) String() string {
	switch v {
	case VerdictProceed:
		return "proceed"
	case VerdictPause:
		return "pause"
	case VerdictDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// GateFunc decides whether a tool call should proceed, pause, or be denied.
// It receives the tool name, definition, and the request about to be executed.
type GateFunc func(ctx context.Context, name string, td registry.ToolDefinition, req registry.CallToolRequest) Verdict

// ApprovalFunc is called when a GateFunc returns VerdictPause. It blocks
// until approval is granted (returns true) or denied (returns false).
// Implementations may wait for human input, a webhook callback, etc.
// If the context is cancelled, the function should return false.
type ApprovalFunc func(ctx context.Context, name string, td registry.ToolDefinition, req registry.CallToolRequest) bool

// Config configures the gate middleware.
type Config struct {
	// Gate decides the verdict for each tool call.
	Gate GateFunc
	// Approval is called when Gate returns VerdictPause. If nil, VerdictPause
	// is treated as VerdictDeny.
	Approval ApprovalFunc
	// DenyMessage is the error message returned when a call is denied.
	// Defaults to "tool call denied by gate".
	DenyMessage string
	// PauseTimeoutMessage is the error message when approval times out or is rejected.
	// Defaults to "tool call not approved".
	PauseTimeoutMessage string
}

// Middleware returns a registry.Middleware that gates tool calls.
// The GateFunc is called before each tool execution to decide whether
// to proceed, pause for approval, or deny the call.
func Middleware(config Config) registry.Middleware {
	if config.DenyMessage == "" {
		config.DenyMessage = "tool call denied by gate"
	}
	if config.PauseTimeoutMessage == "" {
		config.PauseTimeoutMessage = "tool call not approved"
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			verdict := config.Gate(ctx, name, td, req)

			switch verdict {
			case VerdictProceed:
				return next(ctx, req)

			case VerdictPause:
				if config.Approval == nil {
					return registry.MakeErrorResult(
						fmt.Sprintf("[GATE_DENIED] %s (no approval handler configured)", config.DenyMessage),
					), nil
				}
				approved := config.Approval(ctx, name, td, req)
				if !approved {
					return registry.MakeErrorResult(
						fmt.Sprintf("[GATE_DENIED] %s", config.PauseTimeoutMessage),
					), nil
				}
				return next(ctx, req)

			case VerdictDeny:
				return registry.MakeErrorResult(
					fmt.Sprintf("[GATE_DENIED] %s", config.DenyMessage),
				), nil

			default:
				return registry.MakeErrorResult(
					fmt.Sprintf("[GATE_DENIED] unknown verdict: %d", verdict),
				), nil
			}
		}
	}
}

// AlwaysProceed is a GateFunc that allows all tool calls.
func AlwaysProceed(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
	return VerdictProceed
}

// DenyWrites is a GateFunc that denies write operations and allows reads.
func DenyWrites(_ context.Context, _ string, td registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
	if td.IsWrite {
		return VerdictDeny
	}
	return VerdictProceed
}

// PauseWrites is a GateFunc that pauses write operations for approval.
func PauseWrites(_ context.Context, _ string, td registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
	if td.IsWrite {
		return VerdictPause
	}
	return VerdictProceed
}
