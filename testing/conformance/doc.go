//go:build !official_sdk

// Package conformance provides the "everything-server" for MCP conformance testing.
//
// The everything-server implements all testable MCP capabilities so the official
// MCP conformance suite (https://github.com/modelcontextprotocol/conformance)
// can validate mcpkit against the protocol specification. It covers tools,
// resources, prompts, logging, completions, sampling, and elicitation.
package conformance
