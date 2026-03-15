package security

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TenantContext holds multi-tenant identity information for a request.
type TenantContext struct {
	TenantID  string
	UserID    string
	AgentID   string
	SessionID string
}

// tenantKey is the unexported context key type for TenantContext values.
type tenantKey struct{}

// WithTenant returns a context with the given TenantContext stored as a value.
func WithTenant(ctx context.Context, tc TenantContext) context.Context {
	return context.WithValue(ctx, tenantKey{}, tc)
}

// GetTenant retrieves the TenantContext from the context.
// The second return value is false if no TenantContext has been set.
func GetTenant(ctx context.Context) (TenantContext, bool) {
	tc, ok := ctx.Value(tenantKey{}).(TenantContext)
	return tc, ok
}

// TenantExtractor extracts a TenantContext from an incoming request and its context.
// Implementations may inspect HTTP headers, JWT claims, or other request metadata.
type TenantExtractor func(ctx context.Context, req registry.CallToolRequest) TenantContext

// TenantMiddleware returns a registry.Middleware that extracts tenant information
// from each request using the provided extractor and injects it into the context
// via WithTenant. Downstream middleware (such as AuditMiddleware) can retrieve
// the tenant with GetTenant.
func TenantMiddleware(extract TenantExtractor) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			if extract != nil {
				tc := extract(ctx, req)
				ctx = WithTenant(ctx, tc)
			}
			return next(ctx, req)
		}
	}
}
