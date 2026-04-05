package protocol

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCancellationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *CancellationError
		want string
	}{
		{
			name: "basic",
			err:  &CancellationError{},
			want: "request cancelled",
		},
		{
			name: "with reason",
			err:  &CancellationError{Reason: "client disconnected"},
			want: "request cancelled: client disconnected",
		},
		{
			name: "with cause",
			err:  &CancellationError{Cause: context.Canceled},
			want: "request cancelled (context canceled)",
		},
		{
			name: "with reason and cause",
			err:  &CancellationError{Reason: "timeout", Cause: context.DeadlineExceeded},
			want: "request cancelled: timeout (context deadline exceeded)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCancellationError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := context.Canceled
	ce := &CancellationError{Cause: cause}

	if !errors.Is(ce, context.Canceled) {
		t.Error("expected CancellationError to unwrap to context.Canceled")
	}
}

func TestCancellationError_Is(t *testing.T) {
	t.Parallel()

	ce := &CancellationError{Reason: "test"}
	other := &CancellationError{Reason: "different"}

	if !errors.Is(ce, other) {
		t.Error("expected CancellationError.Is to match another CancellationError")
	}

	if errors.Is(ce, fmt.Errorf("not a cancellation")) {
		t.Error("expected CancellationError.Is to not match a plain error")
	}
}

func TestIsCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("something"), false},
		{"context.Canceled", context.Canceled, true},
		{"CancellationError", &CancellationError{Reason: "test"}, true},
		{"wrapped CancellationError", fmt.Errorf("wrap: %w", &CancellationError{Reason: "test"}), true},
		{"wrapped context.Canceled", fmt.Errorf("wrap: %w", context.Canceled), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsCancellation(tt.err); got != tt.want {
				t.Errorf("IsCancellation(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestWrapCancellation(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		if got := WrapCancellation(nil); got != nil {
			t.Errorf("WrapCancellation(nil) = %v, want nil", got)
		}
	})

	t.Run("context.Canceled wraps", func(t *testing.T) {
		t.Parallel()
		got := WrapCancellation(context.Canceled)
		var ce *CancellationError
		if !errors.As(got, &ce) {
			t.Errorf("WrapCancellation(context.Canceled) did not return CancellationError, got %T", got)
		}
	})

	t.Run("non-cancellation passes through", func(t *testing.T) {
		t.Parallel()
		original := errors.New("not cancelled")
		got := WrapCancellation(original)
		if got != original {
			t.Errorf("WrapCancellation returned different error for non-cancellation")
		}
	})
}

func TestToJSONRPCError(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		if got := ToJSONRPCError(nil); got != nil {
			t.Errorf("ToJSONRPCError(nil) = %v, want nil", got)
		}
	})

	t.Run("CancellationError", func(t *testing.T) {
		t.Parallel()
		ce := &CancellationError{Reason: "user abort"}
		got := ToJSONRPCError(ce)
		if got.Code != CodeRequestCancelled {
			t.Errorf("Code = %d, want %d", got.Code, CodeRequestCancelled)
		}
		if got.Message != "Request cancelled: user abort" {
			t.Errorf("Message = %q, unexpected", got.Message)
		}
	})

	t.Run("CancellationError without reason", func(t *testing.T) {
		t.Parallel()
		ce := &CancellationError{}
		got := ToJSONRPCError(ce)
		if got.Code != CodeRequestCancelled {
			t.Errorf("Code = %d, want %d", got.Code, CodeRequestCancelled)
		}
		if got.Message != "Request cancelled" {
			t.Errorf("Message = %q, want 'Request cancelled'", got.Message)
		}
	})

	t.Run("context.Canceled", func(t *testing.T) {
		t.Parallel()
		got := ToJSONRPCError(context.Canceled)
		if got.Code != CodeRequestCancelled {
			t.Errorf("Code = %d, want %d", got.Code, CodeRequestCancelled)
		}
	})

	t.Run("context.DeadlineExceeded", func(t *testing.T) {
		t.Parallel()
		got := ToJSONRPCError(context.DeadlineExceeded)
		if got.Code != CodeRequestCancelled {
			t.Errorf("Code = %d, want %d", got.Code, CodeRequestCancelled)
		}
		if got.Message != "Request timed out" {
			t.Errorf("Message = %q, want 'Request timed out'", got.Message)
		}
	})

	t.Run("generic error becomes InternalError", func(t *testing.T) {
		t.Parallel()
		got := ToJSONRPCError(errors.New("something went wrong"))
		if got.Code != CodeInternalError {
			t.Errorf("Code = %d, want %d", got.Code, CodeInternalError)
		}
	})
}
