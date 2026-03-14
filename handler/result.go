package handler

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Structured error codes for programmatic categorization.
const (
	ErrClientInit   = "CLIENT_INIT_FAILED"
	ErrInvalidParam = "INVALID_PARAM"
	ErrTimeout      = "TIMEOUT"
	ErrNotFound     = "NOT_FOUND"
	ErrAPIError     = "API_ERROR"
	ErrPermission   = "PERMISSION_DENIED"
)

// TextResult creates a text result for a tool response.
func TextResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

// ErrorResult creates an error result for a tool response.
func ErrorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// JSONResult creates a JSON result for a tool response.
func JSONResult(data interface{}) *mcp.CallToolResult {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ErrorResult(err)
	}
	return mcp.NewToolResultText(string(bytes))
}

// CodedErrorResult creates an error result with a structured error code prefix.
// Format: "[ERROR_CODE] message"
func CodedErrorResult(code string, err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("[%s] %s", code, err.Error()))
}

// ActionableErrorResult creates an error result with suggestions for resolution.
func ActionableErrorResult(code string, err error, suggestions ...string) *mcp.CallToolResult {
	msg := fmt.Sprintf("[%s] %s", code, err.Error())
	if len(suggestions) > 0 {
		msg += "\n\nSuggested actions:"
		for _, s := range suggestions {
			msg += "\n  • " + s
		}
	}
	return mcp.NewToolResultError(msg)
}

// ObjectOutputSchema creates an output schema for tools returning JSON objects.
func ObjectOutputSchema(properties map[string]interface{}, required []string) *mcp.ToolOutputSchema {
	return &mcp.ToolOutputSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}
