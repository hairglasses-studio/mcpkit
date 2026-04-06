// Package adapter defines the ProtocolAdapter interface for multi-protocol
// agent gateway support.
//
// Each adapter translates between MCP and another protocol (A2A, gRPC, OpenAPI,
// etc.), enabling the gateway to aggregate tools from heterogeneous backends
// through a unified MCP interface. The package includes concrete adapters for
// A2A and OpenAPI protocols.
//
// Implementations must be safe for concurrent use.
package adapter
