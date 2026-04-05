package registry

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/protocol"
)

// CancellationMiddleware returns middleware that detects context cancellation
// during tool execution and returns a user-friendly error result.
//
// When a tool handler's context is cancelled (e.g., because the client sent
// a notifications/cancelled message), this middleware intercepts the
// context.Canceled error and returns a CallToolResult with IsError=true and
// a descriptive message. This prevents raw context errors from bubbling up
// as opaque internal errors.
//
// The underlying mcp-go server handles the notifications/cancelled -> context
// cancel mapping automatically. This middleware provides the mcpkit layer's
// response to that cancellation.
func CancellationMiddleware() Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
			result, err := next(ctx, request)

			if err != nil && protocol.IsCancellation(err) {
				wrapped := protocol.WrapCancellation(err)
				return MakeErrorResult("Request cancelled"), wrapped
			}

			return result, err
		}
	}
}

// cancellationKey is the context key for storing a cancellation reason.
type cancellationKey struct{}

// WithCancellationReason returns a context annotated with a cancellation reason.
// When the context is subsequently cancelled, middleware can extract this
// reason for logging or error messages.
func WithCancellationReason(ctx context.Context, reason string) context.Context {
	return context.WithValue(ctx, cancellationKey{}, reason)
}

// GetCancellationReason extracts the cancellation reason from the context.
// Returns empty string if no reason was set.
func GetCancellationReason(ctx context.Context) string {
	reason, _ := ctx.Value(cancellationKey{}).(string)
	return reason
}
