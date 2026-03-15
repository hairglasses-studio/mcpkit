package handoff

import "context"

type delegationDepthKey struct{}

// DelegationDepth returns the current delegation depth from the context.
// Returns 0 if no depth has been set.
func DelegationDepth(ctx context.Context) int {
	d, _ := ctx.Value(delegationDepthKey{}).(int)
	return d
}

// WithDelegationDepth returns a new context with the given delegation depth.
func WithDelegationDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, delegationDepthKey{}, depth)
}
