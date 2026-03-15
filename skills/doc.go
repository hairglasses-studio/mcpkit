// Package skills provides context-aware lazy tool loading for MCP agents.
// Tools are grouped into named skill bundles, each with optional trigger
// conditions that activate the bundle based on request context or metadata.
// Bundles are loaded on demand, keeping the active tool surface small and
// reducing unnecessary capability exposure.
package skills
