//go:build !official_sdk

package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// contextKey is an unexported type to prevent collision with other packages
// that may store values under the same logical name.
type contextKey struct{}

// correlationIDKey is the context key for the per-request correlation ID.
var correlationIDKey = contextKey{}

// Config controls correlation-middleware behavior. The zero value is a
// no-op pass-through; populate at least one field to activate.
type Config struct {
	// Logger, when non-nil, is wrapped with a `slog.Logger.With(field=ID)`
	// derived logger and stored under the same context. Downstream handlers
	// that pull the logger via slog.Default() or a context accessor get
	// the correlation-enriched logger automatically.
	Logger *slog.Logger

	// HeaderField is the slog attribute key for the correlation ID. Default
	// "correlation_id". Use "request_id" to match industry convention.
	HeaderField string

	// GenerateID, when non-nil, overrides the default 16-hex-char random ID
	// generator. Hook this if you want to forward an upstream HTTP header
	// (e.g., from a Cloudflare or AWS load balancer) into the middleware.
	GenerateID func(ctx context.Context, req registry.CallToolRequest) string
}

// Middleware returns a registry.Middleware that injects a correlation ID
// into the context for every tool call. The returned middleware is safe
// for concurrent use by multiple handlers.
//
// When cfg is the zero value, the middleware still injects an ID but
// doesn't enrich any logger (FromContext returns the ID, but no logger
// is exposed). When cfg.Logger is non-nil, the enriched logger is
// retrievable via LoggerFromContext.
func Middleware(cfg Config) registry.Middleware {
	field := cfg.HeaderField
	if field == "" {
		field = "correlation_id"
	}
	genID := cfg.GenerateID
	if genID == nil {
		genID = func(_ context.Context, _ registry.CallToolRequest) string { return generateRandomID() }
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			id := genID(ctx, req)
			ctx = context.WithValue(ctx, correlationIDKey, id)
			if cfg.Logger != nil {
				enriched := cfg.Logger.With(slog.String(field, id), slog.String("tool", name))
				ctx = withLogger(ctx, enriched)
			}
			return next(ctx, req)
		}
	}
}

// FromContext returns the correlation ID for the current request, or
// the empty string when no correlation middleware is wired.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// loggerKey is the context key for the correlation-enriched slog.Logger.
type loggerKey struct{}

var loggerCtxKey = loggerKey{}

// withLogger stores a correlation-enriched slog.Logger in the context.
func withLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, l)
}

// LoggerFromContext returns the correlation-enriched slog.Logger when the
// middleware was configured with cfg.Logger; otherwise returns
// slog.Default() so handlers can use it unconditionally.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// generateRandomID produces a 16-character hex correlation ID using
// crypto/rand. Sufficient entropy (64 bits) for any practical
// per-tenant uniqueness without the verbosity of full UUIDs.
func generateRandomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on a healthy system; if it somehow does,
		// fall back to a deterministic-but-unique-enough marker rather
		// than panic in the request hot path.
		return "rand-err"
	}
	return hex.EncodeToString(b[:])
}
