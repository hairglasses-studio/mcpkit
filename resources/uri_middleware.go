//go:build !official_sdk

package resources

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/sanitize"
)

// URIValidationMiddleware returns a Middleware that validates the incoming
// resource request URI against the given policy before passing to the next
// handler. If the URI is invalid the handler returns an error immediately.
//
// Example usage:
//
//	reg := resources.NewResourceRegistry(resources.Config{
//	    Middleware: []resources.Middleware{
//	        resources.URIValidationMiddleware(sanitize.DefaultURIPolicy()),
//	    },
//	})
func URIValidationMiddleware(policy sanitize.URIPolicy) Middleware {
	return func(uri string, rd ResourceDefinition, next ResourceHandlerFunc) ResourceHandlerFunc {
		return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			requestURI := req.Params.URI
			if requestURI == "" {
				// Fall back to the registered URI when the request carries none
				// (e.g. during direct wrapHandler tests).
				requestURI = uri
			}

			if _, err := sanitize.ValidateURI(requestURI, policy); err != nil {
				return nil, fmt.Errorf("resource URI validation failed: %w", err)
			}

			return next(ctx, req)
		}
	}
}
