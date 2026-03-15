package sampling

import (
	"context"
	"errors"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// SamplingClient requests LLM completions from the connected MCP client.
type SamplingClient interface {
	CreateMessage(ctx context.Context, req CreateMessageRequest) (*CreateMessageResult, error)
}

// Type aliases via registry compat layer.
type (
	CreateMessageRequest = registry.CreateMessageRequest
	CreateMessageResult  = registry.CreateMessageResult
	SamplingMessage      = registry.SamplingMessage
	CreateMessageParams  = registry.CreateMessageParams
	ModelPreferences     = registry.ModelPreferences
)

// ErrSamplingUnavailable is returned when no SamplingClient is available in the context.
var ErrSamplingUnavailable = errors.New("sampling: no client available in context")

type samplingKey struct{}

// WithSamplingClient returns a context with the given SamplingClient attached.
func WithSamplingClient(ctx context.Context, client SamplingClient) context.Context {
	return context.WithValue(ctx, samplingKey{}, client)
}

// ClientFromContext extracts the SamplingClient from the context, or nil if none.
func ClientFromContext(ctx context.Context) SamplingClient {
	c, _ := ctx.Value(samplingKey{}).(SamplingClient)
	return c
}
