//go:build !official_sdk

// Package hitools provides human interaction MCP tools built on mcpkit's
// elicitation primitives.
//
// The primary tool, request_human_input, lets an agent ask the connected human
// a question via the MCP elicitation protocol. It supports free-text, yes/no,
// and multiple-choice response formats with configurable urgency and timeout.
// An approval subsystem enables human-in-the-loop confirmation flows for
// sensitive operations.
package hitools
