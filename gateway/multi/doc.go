// Package multi implements a multi-protocol HTTP gateway for mcpkit.
//
// It defines the Adapter interface for translating between agent protocols
// (MCP, A2A, OpenAI function calling, etc.) and a canonical request/response
// model. Each protocol gets a single adapter that implements Detect, Decode,
// and Encode. The Router composes adapters and dispatches incoming HTTP
// requests to the correct one based on protocol auto-detection.
//
// This package is the multi-protocol superset of the single-protocol gateway
// package. Operators who only need MCP aggregation continue using the parent
// gateway package; those who need multi-protocol support import this one.
package multi
