package protocol

import (
	"context"
	"errors"
)

// CancellationError wraps a context error to signal that a request was cancelled
// and should result in a REQUEST_CANCELLED (-32800) JSON-RPC error response.
//
// Tool handlers that respect context cancellation will have their context
// cancelled when the client sends a notifications/cancelled message. The
// CancellationMiddleware in the registry package checks for this error type
// and maps it to the correct MCP error code.
type CancellationError struct {
	// RequestID is the ID of the cancelled request, if known.
	RequestID any
	// Reason is an optional human-readable cancellation reason.
	Reason string
	// Cause is the underlying context error.
	Cause error
}

// Error implements the error interface.
func (e *CancellationError) Error() string {
	msg := "request cancelled"
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Cause != nil {
		msg += " (" + e.Cause.Error() + ")"
	}
	return msg
}

// Unwrap returns the underlying cause for errors.Is/As chain traversal.
func (e *CancellationError) Unwrap() error {
	return e.Cause
}

// Is reports whether target is a CancellationError.
func (e *CancellationError) Is(target error) bool {
	_, ok := target.(*CancellationError)
	return ok
}

// IsCancellation reports whether err represents a cancelled request.
// It returns true for CancellationError, context.Canceled, and
// context.DeadlineExceeded.
func IsCancellation(err error) bool {
	if err == nil {
		return false
	}
	var ce *CancellationError
	if errors.As(err, &ce) {
		return true
	}
	return errors.Is(err, context.Canceled)
}

// WrapCancellation converts a context error into a CancellationError if the
// error represents a cancellation. Returns nil for nil errors and returns the
// original error unchanged if it is not a cancellation.
func WrapCancellation(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return &CancellationError{Cause: err}
	}
	return err
}

// ToJSONRPCError converts a CancellationError to a JSON-RPC Error with the
// correct REQUEST_CANCELLED code. For non-cancellation errors, it returns an
// INTERNAL_ERROR.
func ToJSONRPCError(err error) *Error {
	if err == nil {
		return nil
	}

	var ce *CancellationError
	if errors.As(err, &ce) {
		msg := "Request cancelled"
		if ce.Reason != "" {
			msg += ": " + ce.Reason
		}
		return NewError(CodeRequestCancelled, msg)
	}

	if errors.Is(err, context.Canceled) {
		return NewError(CodeRequestCancelled, "Request cancelled")
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(CodeRequestCancelled, "Request timed out")
	}

	return NewError(CodeInternalError, err.Error())
}
