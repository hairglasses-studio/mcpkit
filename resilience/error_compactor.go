//go:build !official_sdk

package resilience

import (
	"context"
	"fmt"
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ErrorCompactorConfig configures the error compactor middleware.
type ErrorCompactorConfig struct {
	// Threshold is the number of consecutive errors for the same tool
	// before the compactor starts summarizing. Default 3.
	Threshold int
}

// DefaultErrorCompactorConfig returns sensible defaults.
func DefaultErrorCompactorConfig() ErrorCompactorConfig {
	return ErrorCompactorConfig{
		Threshold: 3,
	}
}

// errorTracker tracks consecutive errors per tool.
type errorTracker struct {
	mu     sync.Mutex
	counts map[string]int    // tool name -> consecutive error count
	last   map[string]string // tool name -> last error message
}

func newErrorTracker() *errorTracker {
	return &errorTracker{
		counts: make(map[string]int),
		last:   make(map[string]string),
	}
}

// recordError increments the consecutive error count for a tool.
// Returns the current count and the last error message.
func (et *errorTracker) recordError(tool string, errMsg string) (int, string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.counts[tool]++
	prev := et.last[tool]
	et.last[tool] = errMsg
	return et.counts[tool], prev
}

// recordSuccess resets the consecutive error count for a tool.
func (et *errorTracker) recordSuccess(tool string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.counts[tool] = 0
	delete(et.last, tool)
}

// count returns the current consecutive error count for a tool.
func (et *errorTracker) count(tool string) int {
	et.mu.Lock()
	defer et.mu.Unlock()
	return et.counts[tool]
}

// ErrorCompactorMiddleware returns a registry.Middleware that tracks consecutive
// errors per tool and summarizes repeated errors instead of letting them grow
// the context window linearly. After the configured threshold of consecutive
// errors for the same tool, the middleware replaces the error content with a
// compact summary showing the error count and the latest message.
func ErrorCompactorMiddleware(config ...ErrorCompactorConfig) registry.Middleware {
	cfg := DefaultErrorCompactorConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 3
	}

	tracker := newErrorTracker()

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)

			// If handler returned a Go error, pass through (shouldn't happen per MCP contract).
			if err != nil {
				return result, err
			}

			// Track errors and successes.
			if result != nil && registry.IsResultError(result) {
				errMsg := ""
				if len(result.Content) > 0 {
					if text, ok := registry.ExtractTextContent(result.Content[0]); ok {
						errMsg = text
					}
				}

				count, _ := tracker.recordError(name, errMsg)
				if count >= cfg.Threshold {
					return registry.MakeErrorResult(
						fmt.Sprintf("[ERROR_COMPACTED] tool %q has failed %d consecutive times. Latest: %s", name, count, errMsg),
					), nil
				}
			} else {
				tracker.recordSuccess(name)
			}

			return result, nil
		}
	}
}

// ErrorCompactorTracker exposes the error tracker for testing and monitoring.
// Call ConsecutiveErrors to check the current error count for a tool.
type ErrorCompactorTracker struct {
	tracker *errorTracker
}

// NewErrorCompactorTracker creates a tracker that can be shared with the middleware.
func NewErrorCompactorTracker() *ErrorCompactorTracker {
	return &ErrorCompactorTracker{tracker: newErrorTracker()}
}

// ConsecutiveErrors returns the current consecutive error count for a tool.
func (ect *ErrorCompactorTracker) ConsecutiveErrors(tool string) int {
	return ect.tracker.count(tool)
}

// ErrorCompactorMiddlewareWithTracker is like ErrorCompactorMiddleware but
// uses an externally provided tracker for monitoring and testing.
func ErrorCompactorMiddlewareWithTracker(tracker *ErrorCompactorTracker, config ...ErrorCompactorConfig) registry.Middleware {
	cfg := DefaultErrorCompactorConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 3
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)
			if err != nil {
				return result, err
			}

			if result != nil && registry.IsResultError(result) {
				errMsg := ""
				if len(result.Content) > 0 {
					if text, ok := registry.ExtractTextContent(result.Content[0]); ok {
						errMsg = text
					}
				}

				count, _ := tracker.tracker.recordError(name, errMsg)
				if count >= cfg.Threshold {
					return registry.MakeErrorResult(
						fmt.Sprintf("[ERROR_COMPACTED] tool %q has failed %d consecutive times. Latest: %s", name, count, errMsg),
					), nil
				}
			} else {
				tracker.tracker.recordSuccess(name)
			}

			return result, nil
		}
	}
}
