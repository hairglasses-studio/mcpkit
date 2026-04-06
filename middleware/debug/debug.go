//go:build !official_sdk

package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// correlationIDKey is the context key for the request correlation ID.
var correlationIDKey = contextKey{}

// requestCounter provides a monotonically increasing request number for
// lightweight correlation when full UUIDs are unnecessary.
var requestCounter atomic.Uint64

// DefaultRedactFields lists parameter names that are redacted by default.
var DefaultRedactFields = []string{
	"password",
	"token",
	"secret",
	"api_key",
	"credential",
	"authorization",
}

// Config controls debug middleware behavior.
type Config struct {
	// Logger is the slog.Logger to use. nil defaults to slog.Default().
	Logger *slog.Logger

	// Enabled activates the middleware. When false the middleware is a no-op
	// passthrough with zero overhead.
	Enabled bool

	// LogParams controls whether tool input parameters are logged.
	// Set to false to suppress parameter logging entirely.
	LogParams bool

	// LogResults controls whether tool output content is logged.
	// Outputs can be large; content is truncated to MaxResultLogBytes.
	LogResults bool

	// MaxResultLogBytes limits how many bytes of result content to log.
	// Default: 1024.
	MaxResultLogBytes int

	// RedactFields lists JSON field names whose values are replaced with
	// "[REDACTED]" in parameter logs. Matching is case-insensitive.
	RedactFields []string
}

// DefaultConfig returns a Config initialized from environment variables.
// MCPKIT_DEBUG=1 or MCPKIT_LOG_LEVEL=debug enables the middleware.
func DefaultConfig() Config {
	enabled := os.Getenv("MCPKIT_DEBUG") == "1" ||
		strings.EqualFold(os.Getenv("MCPKIT_LOG_LEVEL"), "debug")

	return Config{
		Logger:            nil, // uses slog.Default()
		Enabled:           enabled,
		LogParams:         true,
		LogResults:        true,
		MaxResultLogBytes: 1024,
		RedactFields:      append([]string(nil), DefaultRedactFields...),
	}
}

// CorrelationID retrieves the request correlation ID from the context.
// Returns an empty string if no debug middleware is active.
func CorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// Middleware returns a registry.Middleware that logs tool invocations using
// structured slog output.
//
// When cfg.Enabled is false the returned middleware passes through to the
// next handler unchanged, incurring zero overhead.
func Middleware(cfg Config) registry.Middleware {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Build redact set for O(1) lookup.
	redactSet := make(map[string]struct{}, len(cfg.RedactFields))
	for _, f := range cfg.RedactFields {
		redactSet[strings.ToLower(f)] = struct{}{}
	}

	maxResultBytes := cfg.MaxResultLogBytes
	if maxResultBytes <= 0 {
		maxResultBytes = 1024
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// When disabled, return the handler unchanged — zero overhead.
		if !cfg.Enabled {
			return next
		}

		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Generate sequential correlation ID.
			corrID := formatCounter(requestCounter.Add(1))

			// Inject correlation ID into context for downstream use.
			ctx = context.WithValue(ctx, correlationIDKey, corrID)

			// Build pre-call log attributes.
			attrs := []slog.Attr{
				slog.String("correlation_id", corrID),
				slog.String("tool", name),
			}

			// Log input parameters (redacted).
			if cfg.LogParams {
				args := registry.ExtractArguments(req)
				if len(args) > 0 {
					paramJSON := redactParams(args, redactSet)
					attrs = append(attrs, slog.String("params", paramJSON))
				}
			}

			logger.LogAttrs(ctx, slog.LevelDebug, "tool_call.start", attrs...)

			// Execute the handler and measure time.
			start := time.Now()
			result, err := next(ctx, req)
			duration := time.Since(start)

			// Build post-call log attributes.
			respAttrs := []slog.Attr{
				slog.String("correlation_id", corrID),
				slog.String("tool", name),
				slog.Duration("duration", duration),
				slog.Int64("duration_ms", duration.Milliseconds()),
			}

			if err != nil {
				respAttrs = append(respAttrs,
					slog.String("error", err.Error()),
				)
				logger.LogAttrs(ctx, slog.LevelError, "tool_call.error", respAttrs...)
				return result, err
			}

			if result != nil {
				respAttrs = append(respAttrs,
					slog.Bool("is_error", result.IsError),
					slog.Int("content_blocks", len(result.Content)),
				)

				// Calculate and log output size.
				outputSize := estimateResultSize(result)
				respAttrs = append(respAttrs, slog.Int("output_bytes", outputSize))

				// Optionally log truncated result content.
				if cfg.LogResults && len(result.Content) > 0 {
					preview := truncateResult(result, maxResultBytes)
					respAttrs = append(respAttrs, slog.String("result_preview", preview))
				}
			}

			level := slog.LevelDebug
			if result != nil && result.IsError {
				level = slog.LevelWarn
			}
			logger.LogAttrs(ctx, level, "tool_call.end", respAttrs...)

			return result, nil
		}
	}
}

// Option configures the debug middleware via functional options.
type Option func(*Config)

// WithLogger sets a custom slog.Logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithEnabled explicitly enables or disables the middleware.
func WithEnabled(enabled bool) Option {
	return func(c *Config) { c.Enabled = enabled }
}

// WithLogParams controls whether input parameters are logged.
func WithLogParams(log bool) Option {
	return func(c *Config) { c.LogParams = log }
}

// WithLogResults controls whether output content previews are logged.
func WithLogResults(log bool) Option {
	return func(c *Config) { c.LogResults = log }
}

// WithMaxResultLogBytes sets the maximum bytes of result content to log.
func WithMaxResultLogBytes(n int) Option {
	return func(c *Config) { c.MaxResultLogBytes = n }
}

// WithRedactFields replaces the default list of sensitive field names.
func WithRedactFields(fields []string) Option {
	return func(c *Config) { c.RedactFields = fields }
}

// New returns a registry.Middleware configured with functional options,
// starting from DefaultConfig().
func New(opts ...Option) registry.Middleware {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return Middleware(cfg)
}

// formatCounter creates a short, human-readable correlation ID from a counter.
// Format: "req-000042"
func formatCounter(n uint64) string {
	return fmt.Sprintf("req-%06d", n)
}

// redactParams serializes tool arguments to JSON with sensitive fields replaced
// by "[REDACTED]".
func redactParams(args map[string]any, redactSet map[string]struct{}) string {
	if len(args) == 0 {
		return "{}"
	}

	redacted := make(map[string]any, len(args))
	for k, v := range args {
		if _, sensitive := redactSet[strings.ToLower(k)]; sensitive {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}

	b, err := json.Marshal(redacted)
	if err != nil {
		return `{"_error":"marshal failed"}`
	}
	return string(b)
}

// estimateResultSize returns the approximate byte size of a CallToolResult by
// marshaling each content block.
func estimateResultSize(result *registry.CallToolResult) int {
	if result == nil {
		return 0
	}
	size := 0
	for _, block := range result.Content {
		b, err := json.Marshal(block)
		if err == nil {
			size += len(b)
		}
	}
	return size
}

// truncateResult returns a string preview of the first content block,
// truncated to maxBytes.
func truncateResult(result *registry.CallToolResult, maxBytes int) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	// Prefer extracting text directly for cleaner output.
	if text, ok := registry.ExtractTextContent(result.Content[0]); ok {
		if len(text) > maxBytes {
			return text[:maxBytes] + "...[truncated]"
		}
		return text
	}

	// Fall back to JSON marshaling for non-text content.
	b, err := json.Marshal(result.Content[0])
	if err != nil {
		return "<marshal error>"
	}
	s := string(b)
	if len(s) > maxBytes {
		return s[:maxBytes] + "...[truncated]"
	}
	return s
}
