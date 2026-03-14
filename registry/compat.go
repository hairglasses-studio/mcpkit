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
	Tool             = mcp.Tool
	CallToolRequest  = mcp.CallToolRequest
	CallToolResult   = mcp.CallToolResult
	ToolInputSchema  = mcp.ToolInputSchema
	ToolOutputSchema = mcp.ToolOutputSchema
	ToolAnnotation   = mcp.ToolAnnotation
	TextContent      = mcp.TextContent
	Content          = mcp.Content
	TaskStatus       = mcp.TaskStatus
	Task             = mcp.Task
	TaskSupport      = mcp.TaskSupport
	ToolExecution    = mcp.ToolExecution
)

var (
	NewToolResultText  = mcp.NewToolResultText
	NewToolResultError = mcp.NewToolResultError
)

// Task status constants re-exported for convenience.
const (
	TaskStatusWorking       = mcp.TaskStatusWorking
	TaskStatusInputRequired = mcp.TaskStatusInputRequired
	TaskStatusCompleted     = mcp.TaskStatusCompleted
	TaskStatusFailed        = mcp.TaskStatusFailed
	TaskStatusCancelled     = mcp.TaskStatusCancelled

	TaskSupportForbidden = mcp.TaskSupportForbidden
	TaskSupportOptional  = mcp.TaskSupportOptional
	TaskSupportRequired  = mcp.TaskSupportRequired
)
