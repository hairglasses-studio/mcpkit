//go:build !official_sdk

// Package truncate provides response-size-limiting middleware for mcpkit tool
// invocations.
//
// When a tool response exceeds MaxBytes of text content, the middleware truncates
// the text to fit within budget and appends a guidance message directing the model
// to use a more specific query. Responses under the limit pass through with zero
// overhead (no copy).
//
// Usage:
//
//	mw := truncate.Middleware(truncate.Config{MaxBytes: 4096})
package truncate
