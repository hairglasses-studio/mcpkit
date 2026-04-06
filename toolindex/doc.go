//go:build !official_sdk

// Package toolindex provides discovery meta-tools for MCP tool registries.
//
// When added to a registry, it generates catalog and search tools that allow
// LLM agents to explore available tools at runtime. The catalog tool lists
// tools grouped by category; the search tool matches tool names and descriptions
// against a query. This supports deferred tool loading patterns where the full
// tool set is large and agents should discover relevant tools before calling them.
package toolindex
