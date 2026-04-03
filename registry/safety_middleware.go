//go:build !official_sdk

package registry

import "context"

// SafetyTier classifies tool operations by their potential impact.
type SafetyTier string

const (
	// TierReadonly indicates a tool that only reads data.
	TierReadonly SafetyTier = "readonly"
	// TierMutating indicates a tool that modifies data with recoverable impact.
	TierMutating SafetyTier = "mutating"
	// TierDestructive indicates a tool that modifies data with potentially
	// unrecoverable impact (IsWrite && Complexity == "complex").
	TierDestructive SafetyTier = "destructive"
)

type safetyTierKeyType struct{}

var safetyTierKey safetyTierKeyType

// ClassifySafetyTier determines the safety tier for a tool definition.
func ClassifySafetyTier(td ToolDefinition) SafetyTier {
	if !td.IsWrite {
		return TierReadonly
	}
	if td.Complexity == ComplexityComplex {
		return TierDestructive
	}
	return TierMutating
}

// SafetyTierMiddleware returns a Middleware that classifies each tool call
// into a SafetyTier and stores it in the request context.
func SafetyTierMiddleware() Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		tier := ClassifySafetyTier(td)
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			ctx = context.WithValue(ctx, safetyTierKey, tier)
			return next(ctx, req)
		}
	}
}

// SafetyTierFromContext extracts the SafetyTier from a context.
// Returns TierReadonly if no tier was set.
func SafetyTierFromContext(ctx context.Context) SafetyTier {
	if tier, ok := ctx.Value(safetyTierKey).(SafetyTier); ok {
		return tier
	}
	return TierReadonly
}
