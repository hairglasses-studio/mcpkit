//go:build !official_sdk

// Package debug provides structured-logging middleware for mcpkit tool invocations.
//
// When enabled (via MCPKIT_DEBUG=1 or Config.Enabled), every tool call is logged
// with: tool name, input parameters (with sensitive fields redacted), execution
// time, output size, error status, and a request correlation ID. When disabled
// the middleware is a zero-overhead passthrough.
//
// Usage:
//
//	mw := debug.Middleware(debug.DefaultConfig())
//	reg := registry.NewToolRegistry(registry.Config{
//	    Middleware: []registry.Middleware{mw},
//	})
package debug
