//go:build official_sdk

// Package prompts provides a registry for MCP prompt templates.
//
// The official SDK variant of this package is not yet implemented.
// Prompt types differ between SDKs ([]*PromptMessage vs []PromptMessage,
// pointer vs value arguments), requiring a dedicated implementation.
package prompts
