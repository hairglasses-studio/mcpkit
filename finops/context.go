package finops

import (
	"context"
	"sync/atomic"
)

type usageKey struct{}
type holderKey struct{}

// TokenUsage represents token counts that can be propagated via context.
// It is intended for use by observability bridges that need to read finops
// data from within a tool invocation without a direct Tracker reference.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	Model        string
}

// WithTokenUsage attaches token usage to a context.
func WithTokenUsage(ctx context.Context, usage TokenUsage) context.Context {
	return context.WithValue(ctx, usageKey{}, usage)
}

// TokenUsageFromContext retrieves token usage from context.
// Returns the zero value and false if no usage has been set.
func TokenUsageFromContext(ctx context.Context) (TokenUsage, bool) {
	u, ok := ctx.Value(usageKey{}).(TokenUsage)
	return u, ok
}

// TokenUsageHolder is a mutable container placed on the context by an outer
// middleware (e.g. observability) so that an inner middleware (e.g. finops)
// can write token usage data back through the immutable context boundary.
// It uses atomic pointer swaps so no mutex is needed.
type TokenUsageHolder struct {
	usage atomic.Pointer[TokenUsage]
}

// Store sets the token usage on the holder.
func (h *TokenUsageHolder) Store(u TokenUsage) {
	h.usage.Store(&u)
}

// Load returns the stored token usage, or zero value and false if not set.
func (h *TokenUsageHolder) Load() (TokenUsage, bool) {
	p := h.usage.Load()
	if p == nil {
		return TokenUsage{}, false
	}
	return *p, true
}

// WithTokenUsageHolder attaches a mutable holder to the context so that inner
// middleware layers can write token usage back to an outer observer.
func WithTokenUsageHolder(ctx context.Context, holder *TokenUsageHolder) context.Context {
	return context.WithValue(ctx, holderKey{}, holder)
}

// TokenUsageHolderFromContext retrieves the holder from context.
// Returns nil, false when no holder has been attached.
func TokenUsageHolderFromContext(ctx context.Context) (*TokenUsageHolder, bool) {
	h, ok := ctx.Value(holderKey{}).(*TokenUsageHolder)
	return h, ok
}
