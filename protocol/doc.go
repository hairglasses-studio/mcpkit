// Package protocol provides MCP specification-compliant JSON-RPC 2.0 types,
// error codes, and helpers for building spec-conformant MCP servers.
//
// This package is SDK-independent: it defines its own constants and types so
// that mcpkit consumers do not need to import the underlying MCP SDK directly
// for protocol-level operations. It includes JSON-RPC request/response types,
// standard and MCP-specific error codes, request cancellation support, and
// notification helpers.
package protocol
