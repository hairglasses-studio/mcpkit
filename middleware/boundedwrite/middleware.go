//go:build !official_sdk

package boundedwrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ConfirmTag is the tag value a ToolDefinition must include in its Tags slice
// to signal that the tool requires explicit confirmation before execution.
// This is the canonical way to declare confirm_required: true.
//
// Example:
//
//	registry.ToolDefinition{
//	    Tool:    mcp.NewTool("payment_charge", ...),
//	    IsWrite: true,
//	    Tags:    []string{boundedwrite.ConfirmTag},
//	}
const ConfirmTag = "confirm_required"

// confirmParam is the parameter name callers must set to true to proceed.
const confirmParam = "confirm"

// Middleware returns a registry.Middleware that implements the bounded-write
// confirmation pattern.
//
// When a tool declares confirm_required (via ConfirmTag in its Tags slice),
// the middleware checks the incoming request for a boolean "confirm" parameter:
//
//   - confirm=true  → call proceeds to the next handler
//   - confirm=false → call is rejected with a human-readable prompt
//   - confirm absent → call is rejected with a human-readable prompt
//
// Tools that do not carry ConfirmTag are passed through unchanged.
func Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		if !requiresConfirmation(td) {
			return next
		}

		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			args := registry.ExtractArguments(req)

			confirmed := false
			if args != nil {
				if v, ok := args[confirmParam]; ok {
					confirmed, _ = v.(bool)
				}
			}

			if !confirmed {
				return registry.MakeErrorResult(rejectionMessage(name, td)), nil
			}

			return next(ctx, req)
		}
	}
}

// requiresConfirmation returns true if the tool definition carries ConfirmTag.
func requiresConfirmation(td registry.ToolDefinition) bool {
	for _, tag := range td.Tags {
		if tag == ConfirmTag {
			return true
		}
	}
	return false
}

// rejectionMessage builds the human-readable rejection prompt shown to callers
// when a confirmation-required tool is invoked without confirm=true.
func rejectionMessage(name string, td registry.ToolDefinition) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[CONFIRM_REQUIRED] Tool %q requires explicit confirmation before execution.\n\n",
		name)

	if td.Tool.Description != "" {
		fmt.Fprintf(&b, "This tool will: %s\n\n", td.Tool.Description)
	}

	b.WriteString(
		"To proceed, re-invoke the tool with the additional parameter:\n" +
			"  confirm: true\n\n" +
			"To cancel, do not re-invoke the tool.",
	)

	return b.String()
}

// RequireConfirmation is a helper for ToolModule authors that returns a
// ToolDefinition with the ConfirmTag already appended to Tags. It is
// intentionally additive — existing tags are preserved.
//
// Usage:
//
//	def := registry.ToolDefinition{
//	    Tool:    mcp.NewTool("payment_charge", ...),
//	    IsWrite: true,
//	}
//	def = boundedwrite.RequireConfirmation(def)
func RequireConfirmation(td registry.ToolDefinition) registry.ToolDefinition {
	for _, t := range td.Tags {
		if t == ConfirmTag {
			return td // already tagged — no-op
		}
	}
	td.Tags = append(td.Tags, ConfirmTag)
	return td
}
