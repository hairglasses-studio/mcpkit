//go:build !official_sdk

// Package boundedwrite provides a Stripe-style confirmation middleware for MCP
// tools with financial or destructive side effects.
//
// Tools that declare confirm_required: true in their metadata will be
// intercepted by this middleware. The caller must pass confirm=true in the
// tool parameters to proceed; any other value — or an absent parameter —
// returns a structured rejection error describing the pending action.
//
// Usage:
//
//	reg.RegisterWithServer(s, registry.Config{
//	    Middleware: []registry.Middleware{
//	        boundedwrite.Middleware(),
//	    },
//	})
//
// Declaring a tool as confirmation-required:
//
//	registry.ToolDefinition{
//	    Tool: mcp.NewTool("payment_charge", ...),
//	    IsWrite: true,
//	    Tags: []string{boundedwrite.ConfirmTag},
//	}
package boundedwrite
