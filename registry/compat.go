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

	// Resource types
	Resource             = mcp.Resource
	ResourceTemplate     = mcp.ResourceTemplate
	TextResourceContents = mcp.TextResourceContents
	BlobResourceContents = mcp.BlobResourceContents
	ReadResourceRequest  = mcp.ReadResourceRequest

	// Prompt types
	Prompt          = mcp.Prompt
	PromptArgument  = mcp.PromptArgument
	PromptMessage   = mcp.PromptMessage
	GetPromptRequest = mcp.GetPromptRequest
	GetPromptResult  = mcp.GetPromptResult
	Role             = mcp.Role
)

var (
	NewToolResultText  = mcp.NewToolResultText
	NewToolResultError = mcp.NewToolResultError

	// Resource constructors
	NewResource         = mcp.NewResource
	NewResourceTemplate = mcp.NewResourceTemplate

	// Prompt constructors
	NewPrompt        = mcp.NewPrompt
	NewPromptMessage = mcp.NewPromptMessage
	NewTextContent   = mcp.NewTextContent
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

	RoleUser      = mcp.RoleUser
	RoleAssistant = mcp.RoleAssistant
)
