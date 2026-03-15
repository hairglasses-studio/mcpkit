package handler

import (
	"encoding/json"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Structured error codes for programmatic categorization.
const (
	ErrClientInit   = "CLIENT_INIT_FAILED"
	ErrInvalidParam = "INVALID_PARAM"
	ErrTimeout      = "TIMEOUT"
	ErrNotFound     = "NOT_FOUND"
	ErrAPIError     = "API_ERROR"
	ErrPermission   = "PERMISSION_DENIED"
	ErrValidation   = "OUTPUT_VALIDATION_FAILED"
)

// TextResult creates a text result for a tool response.
func TextResult(text string) *registry.CallToolResult {
	return registry.MakeTextResult(text)
}

// ErrorResult creates an error result for a tool response.
func ErrorResult(err error) *registry.CallToolResult {
	return registry.MakeErrorResult(err.Error())
}

// JSONResult creates a JSON result for a tool response.
func JSONResult(data interface{}) *registry.CallToolResult {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ErrorResult(err)
	}
	return registry.MakeTextResult(string(bytes))
}

// CodedErrorResult creates an error result with a structured error code prefix.
// Format: "[ERROR_CODE] message"
func CodedErrorResult(code string, err error) *registry.CallToolResult {
	return registry.MakeErrorResult(fmt.Sprintf("[%s] %s", code, err.Error()))
}

// ActionableErrorResult creates an error result with suggestions for resolution.
func ActionableErrorResult(code string, err error, suggestions ...string) *registry.CallToolResult {
	msg := fmt.Sprintf("[%s] %s", code, err.Error())
	if len(suggestions) > 0 {
		msg += "\n\nSuggested actions:"
		for _, s := range suggestions {
			msg += "\n  • " + s
		}
	}
	return registry.MakeErrorResult(msg)
}
