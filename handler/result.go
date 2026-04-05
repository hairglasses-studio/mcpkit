package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Structured error codes for programmatic categorization.
//
// The first block contains specific error codes for common failure modes.
// The second block contains generic codes aligned with the Python workspace
// taxonomy (mcp_workspace.runtime.WorkspaceErrorCode) for cross-ecosystem parity.
const (
	ErrClientInit   = "CLIENT_INIT_FAILED"
	ErrInvalidParam = "INVALID_PARAM"
	ErrTimeout      = "TIMEOUT"
	ErrNotFound     = "NOT_FOUND"
	ErrAPIError     = "API_ERROR"
	ErrPermission   = "PERMISSION_DENIED"
	ErrValidation   = "OUTPUT_VALIDATION_FAILED"

	// Generic codes for cross-ecosystem alignment with Python workspace taxonomy.
	ErrInternal      = "INTERNAL"
	ErrRateLimited   = "RATE_LIMITED"
	ErrUpstreamError = "UPSTREAM_ERROR"
	ErrConflict      = "CONFLICT"
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
func JSONResult(data any) *registry.CallToolResult {
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
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("[%s] %s", code, err.Error()))
	if len(suggestions) > 0 {
		msg.WriteString("\n\nSuggested actions:")
		for _, s := range suggestions {
			msg.WriteString("\n  • " + s)
		}
	}
	return registry.MakeErrorResult(msg.String())
}
