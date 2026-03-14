// compat.go — MCP SDK compatibility / migration adapter layer.
//
// Current SDK:  github.com/mark3labs/mcp-go
// Target SDK:   github.com/modelcontextprotocol/go-sdk (when stable)
//
// When the official SDK ships, update the type aliases and constructor vars
// in THIS FILE. Tool modules that import types through mcpkit need zero changes.
package registry

import "github.com/mark3labs/mcp-go/mcp"

type (
	Tool            = mcp.Tool
	CallToolRequest = mcp.CallToolRequest
	CallToolResult  = mcp.CallToolResult
	ToolInputSchema = mcp.ToolInputSchema
	ToolOutputSchema = mcp.ToolOutputSchema
	ToolAnnotation  = mcp.ToolAnnotation
	TextContent     = mcp.TextContent
	Content         = mcp.Content
)

var (
	NewToolResultText  = mcp.NewToolResultText
	NewToolResultError = mcp.NewToolResultError
)
