//go:build !official_sdk

package registry

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func makeTD(name, desc string) ToolDefinition {
	return ToolDefinition{
		Tool: Tool{
			Name:        name,
			Description: desc,
		},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeTextResult("ok"), nil
		},
	}
}

// ---------------------------------------------------------------------------
// Fingerprint tests
// ---------------------------------------------------------------------------

func TestFingerprint_Determinism(t *testing.T) {
	t.Parallel()
	td := makeTD("tool_a", "does something")
	fp1 := Fingerprint(td)
	fp2 := Fingerprint(td)
	if fp1 != fp2 {
		t.Errorf("fingerprint not deterministic: %x != %x", fp1, fp2)
	}
}

func TestFingerprint_SensitiveToDescription(t *testing.T) {
	t.Parallel()
	td1 := makeTD("tool_a", "original description")
	td2 := makeTD("tool_a", "tampered description")
	if Fingerprint(td1) == Fingerprint(td2) {
		t.Error("fingerprints should differ when description changes")
	}
}

func TestFingerprint_SensitiveToName(t *testing.T) {
	t.Parallel()
	td1 := makeTD("tool_a", "same desc")
	td2 := makeTD("tool_b", "same desc")
	if Fingerprint(td1) == Fingerprint(td2) {
		t.Error("fingerprints should differ when name changes")
	}
}

// ---------------------------------------------------------------------------
// Register tests
// ---------------------------------------------------------------------------

func TestIntegrityStore_Register_HappyPath(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	td := makeTD("my_tool", "description")

	if err := store.Register(td); err != nil {
		t.Fatalf("initial register failed: %v", err)
	}

	// registering same definition again must be idempotent
	if err := store.Register(td); err != nil {
		t.Fatalf("re-register same definition failed: %v", err)
	}

	if v := store.Violations(); len(v) != 0 {
		t.Errorf("expected 0 violations, got %d", len(v))
	}
}

func TestIntegrityStore_Register_TamperDetected(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered")

	_ = store.Register(original)

	err := store.Register(tampered)
	if err == nil {
		t.Fatal("expected error on re-registration with different fingerprint")
	}

	violations := store.Violations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Tool != "my_tool" {
		t.Errorf("violation tool = %q, want my_tool", violations[0].Tool)
	}
}

// ---------------------------------------------------------------------------
// Verify tests
// ---------------------------------------------------------------------------

func TestIntegrityStore_Verify_HappyPath(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	td := makeTD("my_tool", "description")
	_ = store.Register(td)

	if v := store.Verify(td); v != nil {
		t.Errorf("expected nil violation for unmodified tool, got %+v", v)
	}
}

func TestIntegrityStore_Verify_UnknownTool(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	td := makeTD("unknown_tool", "desc")

	// not yet registered — Verify should return nil (no opinion)
	if v := store.Verify(td); v != nil {
		t.Errorf("expected nil for unregistered tool, got %+v", v)
	}
}

func TestIntegrityStore_Verify_TamperDetected(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered description")

	_ = store.Register(original)

	v := store.Verify(tampered)
	if v == nil {
		t.Fatal("expected violation for tampered tool, got nil")
	}
	if v.Tool != "my_tool" {
		t.Errorf("violation.Tool = %q, want my_tool", v.Tool)
	}
	if v.Previous == v.Current {
		t.Error("violation should have distinct Previous and Current fingerprints")
	}
	if v.Timestamp.IsZero() {
		t.Error("violation.Timestamp should not be zero")
	}

	// violation must appear in the store
	stored := store.Violations()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored violation, got %d", len(stored))
	}
}

// ---------------------------------------------------------------------------
// Violations() copy semantics
// ---------------------------------------------------------------------------

func TestIntegrityStore_Violations_ReturnsCopy(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	_ = store.Register(makeTD("t", "v1"))
	store.Verify(makeTD("t", "v2")) // induce a violation

	v1 := store.Violations()
	if len(v1) == 0 {
		t.Fatal("expected at least one violation")
	}

	// mutate the returned slice — internal state must not change
	v1[0].Tool = "mutated"

	v2 := store.Violations()
	if v2[0].Tool == "mutated" {
		t.Error("Violations() returned a reference to internal slice, not a copy")
	}
}

// ---------------------------------------------------------------------------
// IntegrityMiddleware tests
// ---------------------------------------------------------------------------

func TestIntegrityMiddleware_NoViolation(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	td := makeTD("my_tool", "description")
	_ = store.Register(td)

	called := false
	onViolation := func(Violation) error {
		called = true
		return errors.New("should not be called")
	}

	mw := IntegrityMiddleware(store, onViolation)
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("next called"), nil
	}

	handler := mw("my_tool", td, next)
	result, err := handler(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("onViolation should not have been called")
	}
	if IsResultError(result) {
		t.Error("result should not be an error")
	}
	if text, _ := ExtractTextContent(result.Content[0]); text != "next called" {
		t.Errorf("expected 'next called', got %q", text)
	}
}

func TestIntegrityMiddleware_ViolationBlocking(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered")
	_ = store.Register(original)

	nextCalled := false
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		nextCalled = true
		return MakeTextResult("should not reach"), nil
	}

	onViolation := func(v Violation) error {
		return errors.New("violation: blocked")
	}

	mw := IntegrityMiddleware(store, onViolation)
	handler := mw("my_tool", tampered, next)
	result, err := handler(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !IsResultError(result) {
		t.Error("expected error result when violation blocks execution")
	}
	if nextCalled {
		t.Error("next should not have been called when violation is blocking")
	}
}

func TestIntegrityMiddleware_ViolationWarningOnly(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()
	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered")
	_ = store.Register(original)

	violationSeen := false
	onViolation := func(v Violation) error {
		violationSeen = true
		return nil // warning-only: allow execution to proceed
	}

	nextCalled := false
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		nextCalled = true
		return MakeTextResult("proceeded"), nil
	}

	mw := IntegrityMiddleware(store, onViolation)
	handler := mw("my_tool", tampered, next)
	result, err := handler(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !violationSeen {
		t.Error("onViolation should have been called")
	}
	if !nextCalled {
		t.Error("next should have been called in warning-only mode")
	}
	if IsResultError(result) {
		t.Error("result should not be an error in warning-only mode")
	}
}

// ---------------------------------------------------------------------------
// Thread safety
// ---------------------------------------------------------------------------

func TestIntegrityStore_ThreadSafety(t *testing.T) {
	t.Parallel()
	store := NewIntegrityStore()

	const numTools = 20
	const goroutines = 50

	// pre-register tools
	for i := range numTools {
		name := "tool_" + string(rune('a'+i))
		_ = store.Register(makeTD(name, "initial"))
	}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range numTools {
				name := "tool_" + string(rune('a'+i))
				// mix of Verify (correct) and Verify (tampered)
				if g%2 == 0 {
					store.Verify(makeTD(name, "initial"))
				} else {
					store.Verify(makeTD(name, "tampered"))
				}
			}
		}(g)
	}

	// concurrent Violations() calls
	wg.Go(func() {
		for range 100 {
			_ = store.Violations()
		}
	})

	wg.Wait() // must not race or deadlock
}
