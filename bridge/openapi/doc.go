// Package openapi bridges OpenAPI v3 specifications to MCP tool registries.
//
// Given an OpenAPI v3 spec (loaded from URL, file, or in-memory), the Bridge
// auto-generates MCP tool definitions from each API operation and proxies tool
// calls to the upstream REST API. This allows any REST API with an OpenAPI spec
// to be exposed as MCP tools without writing per-endpoint handler code.
//
// The bridge supports configurable tool naming (operationId or path-based),
// authentication headers, custom HTTP clients, and request timeouts.
package openapi
