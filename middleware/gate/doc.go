//go:build !official_sdk

// Package gate provides a ToolCallGate middleware that pauses between tool
// selection and execution.
//
// A GateFunc callback decides whether to proceed, pause (waiting for human
// approval), or deny the call entirely. This enables human-in-the-loop
// confirmation flows where write operations or destructive tools require
// explicit approval before execution.
//
// Usage:
//
//	gf := func(ctx context.Context, name string, td registry.ToolDefinition, req registry.CallToolRequest) gate.Verdict {
//	    if td.IsWrite {
//	        return gate.VerdictPause
//	    }
//	    return gate.VerdictProceed
//	}
//	mw := gate.Middleware(gf)
package gate
