//go:build !official_sdk

package registry

import (
	"context"
	"testing"
)

func TestClassifySafetyTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		isWrite    bool
		complexity ToolComplexity
		want       SafetyTier
	}{
		{"readonly simple", false, ComplexitySimple, TierReadonly},
		{"readonly moderate", false, ComplexityModerate, TierReadonly},
		{"readonly complex", false, ComplexityComplex, TierReadonly},
		{"readonly empty complexity", false, "", TierReadonly},
		{"mutating simple", true, ComplexitySimple, TierMutating},
		{"mutating moderate", true, ComplexityModerate, TierMutating},
		{"mutating empty complexity", true, "", TierMutating},
		{"destructive complex", true, ComplexityComplex, TierDestructive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			td := ToolDefinition{
				IsWrite:    tt.isWrite,
				Complexity: tt.complexity,
			}
			got := ClassifySafetyTier(td)
			if got != tt.want {
				t.Errorf("ClassifySafetyTier(%v, %v) = %q, want %q",
					tt.isWrite, tt.complexity, got, tt.want)
			}
		})
	}
}

func TestSafetyTierMiddleware_StoresTierInContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		isWrite    bool
		complexity ToolComplexity
		want       SafetyTier
	}{
		{"readonly", false, ComplexitySimple, TierReadonly},
		{"mutating", true, ComplexityModerate, TierMutating},
		{"destructive", true, ComplexityComplex, TierDestructive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			td := ToolDefinition{
				Tool:       Tool{Name: "test_tool"},
				IsWrite:    tt.isWrite,
				Complexity: tt.complexity,
			}

			var captured SafetyTier
			next := func(ctx context.Context, _ CallToolRequest) (*CallToolResult, error) {
				captured = SafetyTierFromContext(ctx)
				return MakeTextResult("ok"), nil
			}

			mw := SafetyTierMiddleware()
			handler := mw("test_tool", td, next)
			_, err := handler(context.Background(), CallToolRequest{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if captured != tt.want {
				t.Errorf("SafetyTierFromContext = %q, want %q", captured, tt.want)
			}
		})
	}
}

func TestSafetyTierFromContext_Default(t *testing.T) {
	t.Parallel()
	tier := SafetyTierFromContext(context.Background())
	if tier != TierReadonly {
		t.Errorf("expected default TierReadonly, got %q", tier)
	}
}
