//go:build !official_sdk

package registry

import (
	"context"
	"fmt"
)

// SignatureVerificationMiddleware returns a Middleware that verifies tool
// signatures before execution using the provided SignatureStore.
//
// The onFailure callback is invoked when verification fails:
//   - If onFailure returns a non-nil error, the tool is NOT executed and
//     an error result is returned.
//   - If onFailure returns nil (warning-only mode), execution continues.
//
// Unsigned tools (not in the store) are always allowed through.
func SignatureVerificationMiddleware(store *SignatureStore, onFailure func(toolName string, err error) error) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			if err := store.Verify(td); err != nil {
				if blockErr := onFailure(name, err); blockErr != nil {
					return MakeErrorResult(fmt.Sprintf(
						"signature verification failed for tool %q: %v", name, blockErr,
					)), nil
				}
			}
			return next(ctx, req)
		}
	}
}
