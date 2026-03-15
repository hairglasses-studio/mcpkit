package finops

import (
	"context"
	"testing"
)

// TestWithTokenUsage_RoundTrip verifies that values stored with WithTokenUsage
// can be retrieved with TokenUsageFromContext.
func TestWithTokenUsage_RoundTrip(t *testing.T) {
	t.Parallel()

	want := TokenUsage{
		InputTokens:  42,
		OutputTokens: 100,
		Model:        "claude-3-5-sonnet",
	}

	ctx := WithTokenUsage(context.Background(), want)
	got, ok := TokenUsageFromContext(ctx)

	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got.InputTokens != want.InputTokens {
		t.Errorf("InputTokens: expected %d, got %d", want.InputTokens, got.InputTokens)
	}
	if got.OutputTokens != want.OutputTokens {
		t.Errorf("OutputTokens: expected %d, got %d", want.OutputTokens, got.OutputTokens)
	}
	if got.Model != want.Model {
		t.Errorf("Model: expected %q, got %q", want.Model, got.Model)
	}
}

// TestTokenUsageFromContext_Missing verifies that TokenUsageFromContext returns
// false when no usage has been attached to the context.
func TestTokenUsageFromContext_Missing(t *testing.T) {
	t.Parallel()

	_, ok := TokenUsageFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for context with no token usage, got true")
	}
}

// TestWithTokenUsage_ZeroValue verifies that the zero value of TokenUsage is
// returned correctly and ok=true when explicitly stored.
func TestWithTokenUsage_ZeroValue(t *testing.T) {
	t.Parallel()

	ctx := WithTokenUsage(context.Background(), TokenUsage{})
	got, ok := TokenUsageFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true for explicitly stored zero value")
	}
	if got.InputTokens != 0 || got.OutputTokens != 0 || got.Model != "" {
		t.Errorf("expected zero TokenUsage, got %+v", got)
	}
}

// TestTokenUsageHolder_StoreLoad verifies that a stored TokenUsage can be
// retrieved with the expected values.
func TestTokenUsageHolder_StoreLoad(t *testing.T) {
	t.Parallel()

	var h TokenUsageHolder
	want := TokenUsage{InputTokens: 123, OutputTokens: 456}
	h.Store(want)

	got, ok := h.Load()
	if !ok {
		t.Fatal("expected ok=true after Store, got false")
	}
	if got.InputTokens != want.InputTokens {
		t.Errorf("InputTokens: expected %d, got %d", want.InputTokens, got.InputTokens)
	}
	if got.OutputTokens != want.OutputTokens {
		t.Errorf("OutputTokens: expected %d, got %d", want.OutputTokens, got.OutputTokens)
	}
}

// TestTokenUsageHolder_LoadBeforeStore verifies that Load returns false when
// nothing has been stored yet.
func TestTokenUsageHolder_LoadBeforeStore(t *testing.T) {
	t.Parallel()

	var h TokenUsageHolder
	_, ok := h.Load()
	if ok {
		t.Error("expected ok=false before any Store, got true")
	}
}

// TestWithTokenUsageHolder_RoundTrip verifies that a holder attached via
// WithTokenUsageHolder can be retrieved with TokenUsageHolderFromContext.
func TestWithTokenUsageHolder_RoundTrip(t *testing.T) {
	t.Parallel()

	var h TokenUsageHolder
	ctx := WithTokenUsageHolder(context.Background(), &h)

	got, ok := TokenUsageHolderFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true from context with holder, got false")
	}
	if got != &h {
		t.Error("expected retrieved holder to be the same pointer as stored")
	}
}

// TestTokenUsageHolderFromContext_Missing verifies that TokenUsageHolderFromContext
// returns false when no holder has been attached to the context.
func TestTokenUsageHolderFromContext_Missing(t *testing.T) {
	t.Parallel()

	_, ok := TokenUsageHolderFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for context with no holder, got true")
	}
}

// TestTokenUsageHolder_Overwrite verifies that a second Store overwrites the
// first and the latest value is returned by Load.
func TestTokenUsageHolder_Overwrite(t *testing.T) {
	t.Parallel()

	var h TokenUsageHolder
	h.Store(TokenUsage{InputTokens: 10, OutputTokens: 20})
	h.Store(TokenUsage{InputTokens: 99, OutputTokens: 88})

	got, ok := h.Load()
	if !ok {
		t.Fatal("expected ok=true after second Store, got false")
	}
	if got.InputTokens != 99 {
		t.Errorf("InputTokens: expected 99 after overwrite, got %d", got.InputTokens)
	}
	if got.OutputTokens != 88 {
		t.Errorf("OutputTokens: expected 88 after overwrite, got %d", got.OutputTokens)
	}
}

// TestWithTokenUsage_Overwrite verifies that a child context can override a
// parent's token usage.
func TestWithTokenUsage_Overwrite(t *testing.T) {
	t.Parallel()

	parent := WithTokenUsage(context.Background(), TokenUsage{InputTokens: 10})
	child := WithTokenUsage(parent, TokenUsage{InputTokens: 99, Model: "gpt-4"})

	got, ok := TokenUsageFromContext(child)
	if !ok {
		t.Fatal("expected ok=true from child context")
	}
	if got.InputTokens != 99 {
		t.Errorf("expected InputTokens=99 from child, got %d", got.InputTokens)
	}
	if got.Model != "gpt-4" {
		t.Errorf("expected Model=gpt-4 from child, got %q", got.Model)
	}

	// Parent context should be unchanged.
	parentGot, ok := TokenUsageFromContext(parent)
	if !ok {
		t.Fatal("expected ok=true from parent context")
	}
	if parentGot.InputTokens != 10 {
		t.Errorf("expected parent InputTokens=10, got %d", parentGot.InputTokens)
	}
}
