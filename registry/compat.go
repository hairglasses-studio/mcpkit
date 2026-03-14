//go:build !official_sdk

// compat.go — MCP SDK compatibility / migration adapter layer (mcp-go variant).
//
// Current SDK:  github.com/mark3labs/mcp-go
// Target SDK:   github.com/modelcontextprotocol/go-sdk (when stable)
//
// When the official SDK ships, the official_sdk build tag activates compat_official.go
// instead of this file. Tool modules that import types through mcpkit need zero changes.
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

// MakeTextContent constructs a Content value containing text.
// In mcp-go this is a value type; in the official SDK it would be a pointer.
func MakeTextContent(text string) Content {
	return mcp.TextContent{Type: "text", Text: text}
}

// MakeErrorResult creates a CallToolResult marked as an error with text content.
func MakeErrorResult(text string) *CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{MakeTextContent(text)},
		IsError: true,
	}
}

// MakeTextResult creates a CallToolResult with text content.
func MakeTextResult(text string) *CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{MakeTextContent(text)},
	}
}

// IsResultError returns true if the result is marked as an error.
func IsResultError(r *CallToolResult) bool {
	if r == nil {
		return false
	}
	return r.IsError
}

// ExtractTextContent extracts the text from a Content value if it is a TextContent.
// Returns the text and true if successful, or empty string and false otherwise.
func ExtractTextContent(c Content) (string, bool) {
	tc, ok := c.(mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}

// ExtractArguments returns the tool arguments as map[string]interface{}.
// In mcp-go, Arguments is type `any` and needs a type assertion.
// In the official SDK, Arguments is json.RawMessage and needs unmarshaling.
func ExtractArguments(req CallToolRequest) map[string]interface{} {
	if req.Params.Arguments == nil {
		return nil
	}
	args, ok := req.Params.Arguments.(map[string]interface{})
	if !ok {
		return nil
	}
	return args
}

// MakeStructuredResult creates a CallToolResult with both structured content
// and a text representation.
func MakeStructuredResult(content Content, data any) *CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: data,
	}
}

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
