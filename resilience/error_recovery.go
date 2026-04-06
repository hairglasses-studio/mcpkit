package resilience

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ErrorEscalation represents an error that exceeded retry limits.
// It captures the full context of the failure chain for diagnostics
// and downstream escalation handlers.
type ErrorEscalation struct {
	ToolName string
	Error    error
	Retries  int
	Duration time.Duration
}

// ErrorRecoveryConfig configures automatic error recovery behavior.
type ErrorRecoveryConfig struct {
	// MaxRetries is the maximum number of consecutive retries before
	// escalating. Zero means no retries (fail immediately). Default: 3.
	MaxRetries int

	// RetryDelay is the pause between retry attempts. Zero means no delay.
	RetryDelay time.Duration

	// EscalateFunc is called when MaxRetries is exceeded. It receives the
	// final error and the number of retries attempted. Optional.
	EscalateFunc func(escalation ErrorEscalation)

	// FormatError customises error messages for LLM readability. If nil,
	// CompactError is used.
	FormatError func(err error) string

	// ShouldRetry decides whether a given error is worth retrying. If nil,
	// all errors are considered retryable.
	ShouldRetry func(err error) bool
}

// DefaultErrorRecoveryConfig returns sensible defaults for agent-facing
// tool error recovery.
func DefaultErrorRecoveryConfig() ErrorRecoveryConfig {
	return ErrorRecoveryConfig{
		MaxRetries:  3,
		RetryDelay:  0,
		FormatError: CompactError,
	}
}

// CompactError formats an error for inclusion in an LLM context window.
// It includes the error type classification, the message, and actionable
// recovery hints. The output is designed to be small enough that an agent
// can append many of these without blowing its context budget.
func CompactError(err error) string {
	if err == nil {
		return ""
	}

	var b strings.Builder

	// Classify the error for the LLM.
	errType := classifyError(err)
	b.WriteString(fmt.Sprintf("[%s] %s", errType, err.Error()))

	// Add recovery hint based on classification.
	if hint := recoveryHint(errType); hint != "" {
		b.WriteString("\nHint: ")
		b.WriteString(hint)
	}

	return b.String()
}

// classifyError maps an error to a short, machine-readable category.
func classifyError(err error) string {
	if err == nil {
		return "UNKNOWN"
	}

	// Check for context errors first — they have well-known types.
	if errors.Is(err, context.DeadlineExceeded) {
		return "TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELLED"
	}

	// Check for resilience-package sentinel errors.
	if errors.Is(err, ErrCircuitOpen) {
		return "CIRCUIT_OPEN"
	}

	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "rate_limited") || strings.Contains(msg, "429"):
		return "RATE_LIMITED"
	case strings.Contains(msg, "permission") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "403"):
		return "PERMISSION"
	case strings.Contains(msg, "not found") || strings.Contains(msg, "404"):
		return "NOT_FOUND"
	case strings.Contains(msg, "connection") || strings.Contains(msg, "network") || strings.Contains(msg, "dns"):
		return "NETWORK"
	case strings.Contains(msg, "timeout"):
		return "TIMEOUT"
	default:
		return "TRANSIENT"
	}
}

// recoveryHint returns a short, actionable hint for the LLM.
func recoveryHint(errType string) string {
	switch errType {
	case "TIMEOUT":
		return "The operation timed out. Retry with a simpler request or increase the timeout."
	case "CANCELLED":
		return "The request was cancelled. This is usually intentional; no retry needed."
	case "CIRCUIT_OPEN":
		return "The upstream service is temporarily unavailable. Wait before retrying."
	case "RATE_LIMITED":
		return "Rate limit hit. Back off and retry after a short delay."
	case "PERMISSION":
		return "Access denied. Check credentials or permissions before retrying."
	case "NOT_FOUND":
		return "The resource was not found. Verify the identifier and try again."
	case "NETWORK":
		return "A network error occurred. Check connectivity and retry."
	case "TRANSIENT":
		return "A transient error occurred. Retry may succeed."
	default:
		return ""
	}
}

// ErrorRecoveryMiddleware creates middleware that catches tool errors,
// formats them for LLM context, and supports automatic retry with
// configurable back-off. It implements Factor 9 of the 12-Factor Agent
// pattern: "Compact Errors into Context Window".
//
// When a tool handler returns an error result (IsError=true) or a Go-level
// error, the middleware:
//  1. Checks ShouldRetry — if the error is not retryable, returns immediately
//     with the compact-formatted error.
//  2. Retries up to MaxRetries times, sleeping RetryDelay between attempts.
//  3. If all retries fail, calls EscalateFunc (if set) and returns the
//     compact-formatted error result.
//
// The middleware always returns (*CallToolResult, nil) — never (nil, error) —
// honouring the mcpkit handler contract.
func ErrorRecoveryMiddleware(cfg ErrorRecoveryConfig) registry.Middleware {
	if cfg.FormatError == nil {
		cfg.FormatError = CompactError
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			start := time.Now()

			// First attempt.
			result, err := next(ctx, request)
			if !isFailure(result, err) {
				return result, nil
			}

			// Extract the error for classification.
			toolErr := extractError(result, err)

			// Check retryability.
			if cfg.ShouldRetry != nil && !cfg.ShouldRetry(toolErr) {
				return makeCompactErrorResult(cfg.FormatError, name, toolErr), nil
			}

			// Retry loop.
			var lastErr error = toolErr
			for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
				// Respect context cancellation between retries.
				if ctx.Err() != nil {
					lastErr = ctx.Err()
					break
				}

				// Wait before retry (if configured).
				if cfg.RetryDelay > 0 {
					select {
					case <-ctx.Done():
						lastErr = ctx.Err()
						break
					case <-time.After(cfg.RetryDelay):
					}
					// Re-check after wait.
					if ctx.Err() != nil {
						lastErr = ctx.Err()
						break
					}
				}

				result, err = next(ctx, request)
				if !isFailure(result, err) {
					return result, nil
				}
				lastErr = extractError(result, err)
			}

			// All retries exhausted — escalate.
			if cfg.EscalateFunc != nil {
				cfg.EscalateFunc(ErrorEscalation{
					ToolName: name,
					Error:    lastErr,
					Retries:  cfg.MaxRetries,
					Duration: time.Since(start),
				})
			}

			return makeCompactErrorResult(cfg.FormatError, name, lastErr), nil
		}
	}
}

// isFailure returns true if the tool call represents a failure — either
// a Go-level error or an IsError result.
func isFailure(result *registry.CallToolResult, err error) bool {
	if err != nil {
		return true
	}
	return registry.IsResultError(result)
}

// extractError pulls a Go error from either a Go-level error or an
// error result's text content.
func extractError(result *registry.CallToolResult, err error) error {
	if err != nil {
		return err
	}
	if result != nil && result.IsError && len(result.Content) > 0 {
		if text, ok := registry.ExtractTextContent(result.Content[0]); ok {
			return errors.New(text)
		}
	}
	return errors.New("unknown tool error")
}

// makeCompactErrorResult builds an error result with the tool name
// and formatted error suitable for LLM consumption.
func makeCompactErrorResult(formatFn func(error) string, toolName string, err error) *registry.CallToolResult {
	formatted := formatFn(err)
	msg := fmt.Sprintf("Tool %q failed: %s", toolName, formatted)
	return registry.MakeErrorResult(msg)
}
