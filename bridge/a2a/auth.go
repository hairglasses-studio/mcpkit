package a2a

import (
	"context"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// tokenContextKey is the unexported context key for storing auth tokens.
type tokenContextKey struct{}

// TokenFromContext extracts the auth token stored in the context by
// ContextTokenInjector. Returns empty string if no token is present.
func TokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenContextKey{}).(string)
	return token
}

// TokenExtractor extracts auth tokens from incoming request contexts.
type TokenExtractor interface {
	Extract(ctx context.Context) (string, error)
}

// TokenInjector injects auth tokens into outgoing request contexts.
type TokenInjector interface {
	Inject(ctx context.Context, token string) context.Context
}

// AuthConfig configures bridge auth behavior.
type AuthConfig struct {
	// Extractor pulls the token from the incoming context. If nil,
	// BearerTokenExtractor is used.
	Extractor TokenExtractor

	// Injector places the token into the context for downstream handlers.
	// If nil, ContextTokenInjector is used.
	Injector TokenInjector

	// Required rejects unauthenticated requests when true.
	Required bool

	// HeaderName is the context key holding the raw header value.
	// Default: "Authorization".
	HeaderName string

	// TokenPrefix is stripped from the header value to extract the bare token.
	// Default: "Bearer ".
	TokenPrefix string
}

// defaults fills zero-value fields with sensible defaults.
func (c *AuthConfig) defaults() {
	if c.Extractor == nil {
		c.Extractor = &BearerTokenExtractor{
			HeaderName:  c.HeaderName,
			TokenPrefix: c.TokenPrefix,
		}
	}
	if c.Injector == nil {
		c.Injector = &ContextTokenInjector{}
	}
	if c.HeaderName == "" {
		c.HeaderName = "Authorization"
	}
	if c.TokenPrefix == "" {
		c.TokenPrefix = "Bearer "
	}
}

// authHeaderContextKey is the context key used to pass the raw Authorization
// header value from an HTTP layer into the tool handler context. Transport
// middleware (e.g., an HTTP middleware wrapping the bridge) should set this.
type authHeaderContextKey struct{ name string }

// WithAuthHeader returns a context carrying a raw auth header value.
// Name should match AuthConfig.HeaderName (default "Authorization").
func WithAuthHeader(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, authHeaderContextKey{name: name}, value)
}

// BearerTokenExtractor extracts a bearer token from the context. It looks for
// the raw header value stored by WithAuthHeader, strips the configured prefix,
// and returns the bare token.
type BearerTokenExtractor struct {
	// HeaderName overrides the header key. Default: "Authorization".
	HeaderName string

	// TokenPrefix overrides the prefix to strip. Default: "Bearer ".
	TokenPrefix string
}

// Extract implements TokenExtractor.
func (e *BearerTokenExtractor) Extract(ctx context.Context) (string, error) {
	headerName := e.HeaderName
	if headerName == "" {
		headerName = "Authorization"
	}
	prefix := e.TokenPrefix
	if prefix == "" {
		prefix = "Bearer "
	}

	raw, _ := ctx.Value(authHeaderContextKey{name: headerName}).(string)
	if raw == "" {
		return "", nil
	}

	if !strings.HasPrefix(raw, prefix) {
		return "", nil
	}
	return strings.TrimPrefix(raw, prefix), nil
}

// ContextTokenInjector stores tokens in context for downstream use via
// TokenFromContext.
type ContextTokenInjector struct{}

// Inject implements TokenInjector.
func (i *ContextTokenInjector) Inject(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenContextKey{}, token)
}

// AuthMiddleware creates bridge middleware that propagates auth across the
// protocol boundary. It extracts a token from the incoming context, optionally
// rejects unauthenticated requests, and injects the token into the context
// passed to downstream tool handlers.
func AuthMiddleware(cfg AuthConfig) registry.Middleware {
	cfg.defaults()

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			token, err := cfg.Extractor.Extract(ctx)
			if err != nil {
				return registry.MakeErrorResult("auth: token extraction failed: " + err.Error()), nil
			}

			if token == "" && cfg.Required {
				return registry.MakeErrorResult("auth: authentication required"), nil
			}

			if token != "" {
				ctx = cfg.Injector.Inject(ctx, token)
			}

			return next(ctx, req)
		}
	}
}
