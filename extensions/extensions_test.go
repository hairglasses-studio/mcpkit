package extensions

import (
	"fmt"
	"sync"
	"testing"
)

// helpers

func mustRegister(t *testing.T, r *ExtensionRegistry, ext Extension) {
	t.Helper()
	if err := r.Register(ext); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}
}

func resultsByName(results []NegotiationResult) map[string]NegotiationResult {
	m := make(map[string]NegotiationResult, len(results))
	for _, res := range results {
		m[res.Name] = res
	}
	return m
}

func activeByName(exts []Extension) map[string]Extension {
	m := make(map[string]Extension, len(exts))
	for _, ext := range exts {
		m[ext.Name] = ext
	}
	return m
}

// --- Register / Available ---

func TestRegisterAndAvailable(t *testing.T) {
	r := NewExtensionRegistry()

	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0", Description: "Health checks"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0", Description: "Token accounting"})
	mustRegister(t, r, Extension{Name: "mcpkit:tracing", Version: "1.1.0"})

	available := r.Available()
	if len(available) != 3 {
		t.Fatalf("expected 3 available extensions, got %d", len(available))
	}

	names := make(map[string]bool)
	for _, ext := range available {
		names[ext.Name] = true
	}
	for _, want := range []string{"mcpkit:health", "mcpkit:finops", "mcpkit:tracing"} {
		if !names[want] {
			t.Errorf("Available() missing %q", want)
		}
	}
}

func TestAvailableEmptyRegistry(t *testing.T) {
	r := NewExtensionRegistry()
	if got := r.Available(); len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// --- Negotiate all match ---

func TestNegotiateAllMatch(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0"})

	results := r.Negotiate([]string{"mcpkit:health", "mcpkit:finops"})
	byName := resultsByName(results)

	if len(byName) != 2 {
		t.Fatalf("expected 2 results, got %d", len(byName))
	}
	for _, name := range []string{"mcpkit:health", "mcpkit:finops"} {
		res, ok := byName[name]
		if !ok {
			t.Errorf("missing result for %q", name)
			continue
		}
		if !res.Accepted {
			t.Errorf("%q should be accepted", name)
		}
		if res.Reason != "" {
			t.Errorf("%q should have empty reason, got %q", name, res.Reason)
		}
	}
}

// --- Negotiate partial match ---

func TestNegotiatePartialMatch(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:tracing", Version: "1.1.0"})

	// Only offer health and tracing; finops is optional so it won't appear in results.
	results := r.Negotiate([]string{"mcpkit:health", "mcpkit:tracing"})
	byName := resultsByName(results)

	// finops is optional and not offered — should NOT appear in results.
	if _, found := byName["mcpkit:finops"]; found {
		t.Errorf("optional unmatched extension should not appear in results")
	}

	// health and tracing should be accepted.
	for _, name := range []string{"mcpkit:health", "mcpkit:tracing"} {
		res, ok := byName[name]
		if !ok {
			t.Errorf("missing result for %q", name)
			continue
		}
		if !res.Accepted {
			t.Errorf("%q should be accepted", name)
		}
	}
}

// --- Negotiate none match ---

func TestNegotiateNoneMatch(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0"})

	// Offer nothing — all extensions are optional.
	results := r.Negotiate([]string{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty offer with no required exts, got %d", len(results))
	}
}

// --- Required extension not offered ---

func TestNegotiateRequiredNotOffered(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:auth", Version: "2.0.0", Required: true})
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"}) // optional

	results := r.Negotiate([]string{})
	byName := resultsByName(results)

	// mcpkit:auth is required and not offered — should appear as rejected.
	authRes, ok := byName["mcpkit:auth"]
	if !ok {
		t.Fatalf("required extension mcpkit:auth missing from results")
	}
	if authRes.Accepted {
		t.Errorf("required extension not offered should not be accepted")
	}
	if authRes.Reason == "" {
		t.Errorf("rejected required extension should have a non-empty reason")
	}

	// optional health was not offered — should NOT appear.
	if _, found := byName["mcpkit:health"]; found {
		t.Errorf("unmatched optional extension should not appear in results")
	}
}

func TestNegotiateRequiredOffered(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:auth", Version: "2.0.0", Required: true})

	results := r.Negotiate([]string{"mcpkit:auth"})
	byName := resultsByName(results)

	res, ok := byName["mcpkit:auth"]
	if !ok {
		t.Fatalf("mcpkit:auth missing from results")
	}
	if !res.Accepted {
		t.Errorf("required extension that was offered should be accepted")
	}
}

// --- IsActive after negotiation ---

func TestIsActiveAfterNegotiation(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0"})

	r.Negotiate([]string{"mcpkit:health"})

	if !r.IsActive("mcpkit:health") {
		t.Errorf("mcpkit:health should be active after negotiation")
	}
	if r.IsActive("mcpkit:finops") {
		t.Errorf("mcpkit:finops should not be active (not offered)")
	}
	if r.IsActive("mcpkit:unknown") {
		t.Errorf("unknown extension should not be active")
	}
}

// --- Active() returns only negotiated extensions ---

func TestActive(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:finops", Version: "0.2.0"})
	mustRegister(t, r, Extension{Name: "mcpkit:tracing", Version: "1.1.0"})

	r.Negotiate([]string{"mcpkit:health", "mcpkit:tracing"})

	active := r.Active()
	if len(active) != 2 {
		t.Fatalf("expected 2 active extensions, got %d", len(active))
	}

	byName := activeByName(active)
	if _, ok := byName["mcpkit:health"]; !ok {
		t.Errorf("mcpkit:health should be in Active()")
	}
	if _, ok := byName["mcpkit:tracing"]; !ok {
		t.Errorf("mcpkit:tracing should be in Active()")
	}
	if _, ok := byName["mcpkit:finops"]; ok {
		t.Errorf("mcpkit:finops should not be in Active()")
	}
}

func TestActiveBeforeNegotiation(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})

	if active := r.Active(); len(active) != 0 {
		t.Errorf("expected no active extensions before negotiation, got %v", active)
	}
}

// --- Duplicate registration ---

func TestDuplicateRegistration(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "1.0.0"})

	err := r.Register(Extension{Name: "mcpkit:health", Version: "2.0.0"})
	if err == nil {
		t.Errorf("expected error on duplicate registration, got nil")
	}
}

// --- Empty name ---

func TestEmptyName(t *testing.T) {
	r := NewExtensionRegistry()
	err := r.Register(Extension{Name: "", Version: "1.0.0"})
	if err == nil {
		t.Errorf("expected error for empty name, got nil")
	}
}

// --- Metadata preserved ---

func TestMetadataPreserved(t *testing.T) {
	r := NewExtensionRegistry()
	meta := map[string]string{"endpoint": "/health", "format": "json"}
	mustRegister(t, r, Extension{
		Name:     "mcpkit:health",
		Version:  "1.0.0",
		Metadata: meta,
	})

	available := r.Available()
	if len(available) != 1 {
		t.Fatalf("expected 1 extension")
	}
	if available[0].Metadata["endpoint"] != "/health" {
		t.Errorf("metadata not preserved: got %v", available[0].Metadata)
	}
}

// --- NegotiationResult carries correct version ---

func TestNegotiationResultVersion(t *testing.T) {
	r := NewExtensionRegistry()
	mustRegister(t, r, Extension{Name: "mcpkit:health", Version: "3.5.1"})

	results := r.Negotiate([]string{"mcpkit:health"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Version != "3.5.1" {
		t.Errorf("expected version 3.5.1, got %q", results[0].Version)
	}
}

// --- Thread safety ---

func TestConcurrentRegisterAndNegotiate(t *testing.T) {
	r := NewExtensionRegistry()

	// Pre-register a set of extensions so Negotiate has something to work with.
	for i := range 5 {
		name := fmt.Sprintf("mcpkit:ext%d", i)
		_ = r.Register(Extension{Name: name, Version: "1.0.0"})
	}

	var wg sync.WaitGroup

	// Concurrent Register calls (new names that don't collide).
	for i := 5; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("mcpkit:ext%d", idx)
			_ = r.Register(Extension{Name: name, Version: "1.0.0"})
		}(i)
	}

	// Concurrent Negotiate calls.
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			offered := []string{
				fmt.Sprintf("mcpkit:ext%d", idx%5),
			}
			_ = r.Negotiate(offered)
		}(i)
	}

	// Concurrent Available calls.
	for range 5 {
		wg.Go(func() {
			_ = r.Available()
		})
	}

	// Concurrent IsActive calls.
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = r.IsActive(fmt.Sprintf("mcpkit:ext%d", idx))
		}(i)
	}

	// Concurrent Active calls.
	for range 5 {
		wg.Go(func() {
			_ = r.Active()
		})
	}

	wg.Wait()
	// If we reach here without a data race, the test passes.
}
