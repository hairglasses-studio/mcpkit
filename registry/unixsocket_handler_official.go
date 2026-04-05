//go:build official_sdk

// unixsocket_handler_official.go provides a stub for the official SDK variant.
// The official go-sdk does not expose HandleMessage / session management in the
// same way as mcp-go, so Unix socket pooling is not yet supported.
package registry
