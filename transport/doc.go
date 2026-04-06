// Package transport provides a transport abstraction layer for MCP servers.
//
// It defines the Transport interface and concrete adapters for stdio, HTTP,
// WebSocket, and Unix socket transports. Servers can switch between
// communication mechanisms without changing application logic. The package
// also provides transport middleware for logging, metrics, and message
// transformation, composable via the Chain function.
package transport
