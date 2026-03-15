//go:build official_sdk

// compat_official.go — MCP SDK compatibility layer (official go-sdk variant).
//
// SDK: github.com/modelcontextprotocol/go-sdk
//
// This file is activated by the official_sdk build tag and provides the same
// exported API as compat.go but backed by the official SDK types.
package registry

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type (
	Tool            = mcp.Tool
	CallToolRequest = mcp.CallToolRequest
	CallToolResult  = mcp.CallToolResult
	ToolAnnotation  = mcp.ToolAnnotations // plural in official SDK
	TextContent     = mcp.TextContent
	Content         = mcp.Content

	// Resource types
	Resource            = mcp.Resource
	ResourceTemplate    = mcp.ResourceTemplate
	ResourceContents    = mcp.ResourceContents
	ReadResourceRequest = mcp.ReadResourceRequest
	ReadResourceResult  = mcp.ReadResourceResult

	// Prompt types
	Prompt           = mcp.Prompt
	PromptArgument   = mcp.PromptArgument
	PromptMessage    = mcp.PromptMessage
	GetPromptRequest = mcp.GetPromptRequest
	GetPromptResult  = mcp.GetPromptResult
	Role             = mcp.Role

	// Sampling types
	CreateMessageRequest = mcp.CreateMessageRequest
	CreateMessageResult  = mcp.CreateMessageResult
	SamplingMessage      = mcp.SamplingMessage
	CreateMessageParams  = mcp.CreateMessageParams
	ModelPreferences     = mcp.ModelPreferences

	// Root types
	Root             = mcp.Root
	ListRootsParams  = mcp.ListRootsParams
	ListRootsResult  = mcp.ListRootsResult
)

// Note: The official SDK does not have separate ToolInputSchema/ToolOutputSchema
// types — both Tool.InputSchema and Tool.OutputSchema are typed as `any`.
// These wrapper types maintain API compatibility with mcpkit consumers.
type ToolInputSchema = any
type ToolOutputSchema = any

// Note: The official SDK does not have Task types. These are placeholder types
// for forward compatibility. They will be updated when the official SDK adds
// task support.
type TaskStatus string
type TaskSupport string

type Task struct {
	TaskId string
}

type ToolExecution struct {
	TaskSupport TaskSupport
}

// Task status constants (placeholder — official SDK does not yet support tasks).
const (
	TaskStatusWorking       TaskStatus = "working"
	TaskStatusInputRequired TaskStatus = "input_required"
	TaskStatusCompleted     TaskStatus = "completed"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusCancelled     TaskStatus = "cancelled"

	TaskSupportForbidden TaskSupport = ""
	TaskSupportOptional  TaskSupport = "optional"
	TaskSupportRequired  TaskSupport = "required"

	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Note: The official SDK does not have TextResourceContents/BlobResourceContents.
// ResourceContents is a single struct with Text and Blob fields.
// These adapters maintain API compatibility.
type TextResourceContents = mcp.ResourceContents
type BlobResourceContents = mcp.ResourceContents

// ExtractResourceText extracts the text from the first resource content in a ReadResourceResult.
// Returns the text and true if the first content has text.
func ExtractResourceText(result *ReadResourceResult) (string, bool) {
	if result == nil || len(result.Contents) == 0 || result.Contents[0] == nil {
		return "", false
	}
	return result.Contents[0].Text, result.Contents[0].Text != ""
}

// MakeTextContent constructs a Content value containing text.
func MakeTextContent(text string) Content {
	return &mcp.TextContent{Text: text}
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

// ExtractArguments returns the tool arguments as map[string]interface{}.
// In the official SDK, Arguments is json.RawMessage and needs unmarshaling.
func ExtractArguments(req CallToolRequest) map[string]interface{} {
	if req.Params == nil || len(req.Params.Arguments) == 0 {
		return nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil
	}
	return args
}

// GetToolTaskSupport returns the TaskSupport setting from a Tool.
// The official SDK does not support tasks, so this always returns TaskSupportForbidden.
func GetToolTaskSupport(_ Tool) TaskSupport {
	return TaskSupportForbidden
}

// HasTaskParams returns true if the request includes task augmentation params.
// The official SDK does not support tasks, so this always returns false.
func HasTaskParams(_ CallToolRequest) bool {
	return false
}

// ExtractTaskTTL returns the task TTL from the request.
// The official SDK does not support tasks, so this always returns 0.
func ExtractTaskTTL(_ CallToolRequest) int64 {
	return 0
}

// MakeStructuredResult creates a CallToolResult with both structured content
// and a text representation.
func MakeStructuredResult(content Content, data any) *CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{content},
		StructuredContent: data,
	}
}

// ExtractTextContent extracts the text from a Content value if it is a TextContent.
func ExtractTextContent(c Content) (string, bool) {
	tc, ok := c.(*mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}
