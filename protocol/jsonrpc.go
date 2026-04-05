// Package protocol provides MCP specification-compliant JSON-RPC 2.0 types,
// error codes, and helpers for building spec-conformant MCP servers.
//
// This package is SDK-independent: it defines its own constants and types so
// that mcpkit consumers do not need to import the underlying MCP SDK directly
// for protocol-level operations.
package protocol

import (
	"encoding/json"
	"fmt"
)

// JSONRPC version constant.
const JSONRPCVersion = "2.0"

// Standard JSON-RPC 2.0 error codes (spec section 5.1).
const (
	// CodeParseError is returned when the server receives invalid JSON.
	CodeParseError = -32700

	// CodeInvalidRequest is returned when the JSON is valid but not a proper
	// JSON-RPC 2.0 request object.
	CodeInvalidRequest = -32600

	// CodeMethodNotFound is returned when the requested method does not exist.
	CodeMethodNotFound = -32601

	// CodeInvalidParams is returned when the method parameters are invalid.
	CodeInvalidParams = -32602

	// CodeInternalError is returned for internal JSON-RPC errors.
	CodeInternalError = -32603
)

// MCP-specific error codes (MCP spec).
const (
	// CodeRequestCancelled is returned when a request is cancelled via
	// notifications/cancelled.
	CodeRequestCancelled = -32800

	// CodeResourceNotFound is returned when a requested resource URI is not found.
	CodeResourceNotFound = -32002
)

// Error represents a JSON-RPC 2.0 error with code, message, and optional data.
// It implements the error interface and can be used both as a Go error and
// serialized into a JSON-RPC error response.
type Error struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`

	// Message is a short human-readable description of the error.
	Message string `json:"message"`

	// Data carries additional error information. May be nil.
	Data any `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("jsonrpc error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// Is reports whether target matches this error's code, enabling errors.Is
// comparisons between Error values with the same code.
func (e *Error) Is(target error) bool {
	if te, ok := target.(*Error); ok {
		return e.Code == te.Code
	}
	return false
}

// Sentinel errors for each standard code, usable with errors.Is.
var (
	ErrParseError       = &Error{Code: CodeParseError, Message: "Parse error"}
	ErrInvalidRequest   = &Error{Code: CodeInvalidRequest, Message: "Invalid Request"}
	ErrMethodNotFound   = &Error{Code: CodeMethodNotFound, Message: "Method not found"}
	ErrInvalidParams    = &Error{Code: CodeInvalidParams, Message: "Invalid params"}
	ErrInternalError    = &Error{Code: CodeInternalError, Message: "Internal error"}
	ErrRequestCancelled = &Error{Code: CodeRequestCancelled, Message: "Request cancelled"}
	ErrResourceNotFound = &Error{Code: CodeResourceNotFound, Message: "Resource not found"}
)

// NewError creates a new JSON-RPC error with the given code and message.
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewErrorWithData creates a new JSON-RPC error with the given code, message,
// and additional data payload.
func NewErrorWithData(code int, message string, data any) *Error {
	return &Error{Code: code, Message: message, Data: data}
}

// IsStandardCode reports whether code is a standard JSON-RPC 2.0 error code
// (in the range -32700 to -32600) or an MCP-defined code.
func IsStandardCode(code int) bool {
	// Standard JSON-RPC range: -32700 to -32600
	if code >= -32700 && code <= -32600 {
		return true
	}
	// MCP-specific codes
	switch code {
	case CodeRequestCancelled, CodeResourceNotFound:
		return true
	}
	return false
}

// CodeName returns a human-readable name for known error codes.
func CodeName(code int) string {
	switch code {
	case CodeParseError:
		return "ParseError"
	case CodeInvalidRequest:
		return "InvalidRequest"
	case CodeMethodNotFound:
		return "MethodNotFound"
	case CodeInvalidParams:
		return "InvalidParams"
	case CodeInternalError:
		return "InternalError"
	case CodeRequestCancelled:
		return "RequestCancelled"
	case CodeResourceNotFound:
		return "ResourceNotFound"
	default:
		return "Unknown"
	}
}

// Request represents a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether this request is a JSON-RPC notification
// (i.e., has no ID field and should not receive a response).
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// Response represents a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// NewResponse creates a successful JSON-RPC response.
func NewResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates a JSON-RPC error response.
func NewErrorResponse(id any, err *Error) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   err,
	}
}
