//go:build official_sdk

// Package resources provides a registry for MCP resources and resource templates.
//
// The official SDK variant of this package is not yet implemented. Resource
// handler signatures differ fundamentally between SDKs ([]ResourceContents vs
// *ReadResourceResult), requiring a dedicated implementation.
package resources
