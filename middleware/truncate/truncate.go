//go:build !official_sdk

// Package truncate provides response-size-limiting middleware for mcpkit tool invocations.
//
// When a tool response exceeds MaxBytes of text content, the middleware truncates
// the text to fit within budget and appends a guidance message directing the model
// to use a more specific query. Responses under the limit pass through with zero
// overhead (no copy).
//
// Usage:
//
//	mw := truncate.Middleware(truncate.Config{MaxBytes: 4096})
//	reg := registry.NewToolRegistry(registry.Config{
//	    Middleware: []registry.Middleware{mw},
//	})
//
// Or with functional options:
//
//	mw := truncate.New(truncate.WithMaxBytes(8192), truncate.WithMessage("..."))
package truncate

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

const (
	// DefaultMaxBytes is the default maximum text content size in bytes.
	DefaultMaxBytes = 4096

	// DefaultHardMax is the absolute ceiling that MaxBytes cannot exceed.
	DefaultHardMax = 16384

	// DefaultMessage is appended to truncated responses.
	DefaultMessage = "[Output truncated. Use a more specific query or request a subset of results.]"
)

// Config controls truncation middleware behavior.
type Config struct {
	// MaxBytes is the maximum allowed text content size in bytes.
	// Responses with total text content exceeding this are truncated.
	// Default: 4096.
	MaxBytes int

	// HardMax is the absolute ceiling. If MaxBytes is set higher than HardMax,
	// HardMax wins. Default: 16384.
	HardMax int

	// Message is the guidance text appended to truncated responses.
	// Default: DefaultMessage.
	Message string
}

// DefaultConfig returns a Config with production defaults.
func DefaultConfig() Config {
	return Config{
		MaxBytes: DefaultMaxBytes,
		HardMax:  DefaultHardMax,
		Message:  DefaultMessage,
	}
}

// effectiveMax returns the enforced maximum, clamping MaxBytes to HardMax.
func (c Config) effectiveMax() int {
	max := c.MaxBytes
	if max <= 0 {
		max = DefaultMaxBytes
	}
	hard := c.HardMax
	if hard <= 0 {
		hard = DefaultHardMax
	}
	if max > hard {
		max = hard
	}
	return max
}

// Middleware returns a registry.Middleware that truncates tool responses whose
// total text content exceeds cfg.MaxBytes.
//
// When truncation occurs, text content is trimmed to fit within the byte budget
// and a guidance message is appended as an additional text content block.
// Non-text content blocks are passed through unchanged.
// Error responses (IsError == true) are never modified.
func Middleware(cfg Config) registry.Middleware {
	limit := cfg.effectiveMax()

	msg := cfg.Message
	if msg == "" {
		msg = DefaultMessage
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)
			if err != nil {
				return result, err
			}

			// Nil result or error result: pass through unchanged.
			if result == nil || result.IsError {
				return result, nil
			}

			// Measure total text content size.
			totalSize := 0
			for _, block := range result.Content {
				if text, ok := registry.ExtractTextContent(block); ok {
					totalSize += len(text)
				}
			}

			// Under limit: zero-overhead passthrough.
			if totalSize <= limit {
				return result, nil
			}

			// Truncate: rebuild content with text trimmed to fit within budget.
			budget := limit
			truncated := make([]registry.Content, 0, len(result.Content)+1)

			for _, block := range result.Content {
				text, ok := registry.ExtractTextContent(block)
				if !ok {
					// Non-text content passes through unchanged.
					truncated = append(truncated, block)
					continue
				}

				if budget <= 0 {
					// No budget remaining, skip this text block.
					continue
				}

				if len(text) <= budget {
					// Fits within remaining budget.
					truncated = append(truncated, block)
					budget -= len(text)
				} else {
					// Trim to remaining budget.
					truncated = append(truncated, registry.MakeTextContent(text[:budget]))
					budget = 0
				}
			}

			// Append the guidance message.
			truncated = append(truncated, registry.MakeTextContent(msg))

			result.Content = truncated
			return result, nil
		}
	}
}

// Option configures the truncation middleware via functional options.
type Option func(*Config)

// WithMaxBytes sets the maximum text content size in bytes.
func WithMaxBytes(n int) Option {
	return func(c *Config) { c.MaxBytes = n }
}

// WithHardMax sets the absolute ceiling for MaxBytes.
func WithHardMax(n int) Option {
	return func(c *Config) { c.HardMax = n }
}

// WithMessage sets the guidance text appended to truncated responses.
func WithMessage(s string) Option {
	return func(c *Config) { c.Message = s }
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
