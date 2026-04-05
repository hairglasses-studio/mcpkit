//go:build !official_sdk

package gateway

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/resilience"
)

// TestDynamicUpstreamRegistry_EmptyName verifies that Add rejects an empty upstream name.
func TestDynamicUpstreamRegistry_EmptyName(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	_, err := d.Add(context.Background(), UpstreamConfig{Name: "", URL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// TestDynamicUpstreamRegistry_DuplicateAdd verifies that adding the same name twice
// returns ErrDuplicateUpstream on the second call.
func TestDynamicUpstreamRegistry_DuplicateAdd(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "dup", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	// First add should succeed.
	if _, err := d.Add(context.Background(), cfg); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	// Second add with same name should fail with ErrDuplicateUpstream.
	_, err := d.Add(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on duplicate Add")
	}
	if !errors.Is(err, ErrDuplicateUpstream) {
		t.Fatalf("expected ErrDuplicateUpstream, got: %v", err)
	}
}

// TestDynamicUpstreamRegistry_RemoveNotFound verifies that removing a non-existent
// upstream returns ErrUpstreamNotFound.
func TestDynamicUpstreamRegistry_RemoveNotFound(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	err := d.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error for removing non-existent upstream")
	}
	if !errors.Is(err, ErrUpstreamNotFound) {
		t.Fatalf("expected ErrUpstreamNotFound, got: %v", err)
	}
}

// TestDynamicUpstreamRegistry_ListEmpty verifies that a new registry returns an empty list.
func TestDynamicUpstreamRegistry_ListEmpty(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	names := d.List()
	if len(names) != 0 {
		t.Fatalf("expected empty list, got %v", names)
	}
	if d.Len() != 0 {
		t.Fatalf("expected Len 0, got %d", d.Len())
	}
}

// TestDynamicUpstreamRegistry_GetNotFound verifies that Get returns false for unknown names.
func TestDynamicUpstreamRegistry_GetNotFound(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	_, ok := d.Get("missing")
	if ok {
		t.Fatal("expected Get to return false for unknown upstream")
	}
}

// TestDynamicUpstreamRegistry_DefaultPolicy verifies that when a config has a zero
// Policy, the registry's default policy is applied before forwarding to the gateway.
func TestDynamicUpstreamRegistry_DefaultPolicy(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "policied", echoTool("act"))
	gw, _ := NewGateway()
	defer gw.Close()

	defaultPolicy := UpstreamPolicy{
		CallTimeout: 5 * time.Second,
		CircuitBreaker: &resilience.CircuitBreakerConfig{
			FailureThreshold: 3,
			Timeout:          10 * time.Second,
		},
	}

	d := NewDynamicUpstreamRegistry(gw, WithDefaultPolicy(defaultPolicy))

	// Config has zero Policy — default should be applied.
	count, err := d.Add(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one tool")
	}

	stored, ok := d.Get("policied")
	if !ok {
		t.Fatal("expected stored config for 'policied'")
	}
	if stored.Policy.CallTimeout != defaultPolicy.CallTimeout {
		t.Fatalf("expected CallTimeout %v, got %v", defaultPolicy.CallTimeout, stored.Policy.CallTimeout)
	}
	if stored.Policy.CircuitBreaker == nil {
		t.Fatal("expected CircuitBreaker to be set from default policy")
	}
}

// TestDynamicUpstreamRegistry_ExplicitPolicyNotOverridden verifies that an explicit
// policy on the config is not replaced by the default.
func TestDynamicUpstreamRegistry_ExplicitPolicyNotOverridden(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "explicit", echoTool("act"))
	gw, _ := NewGateway()
	defer gw.Close()

	defaultPolicy := UpstreamPolicy{CallTimeout: 5 * time.Second}
	explicitPolicy := UpstreamPolicy{CallTimeout: 99 * time.Second}
	cfg.Policy = explicitPolicy

	d := NewDynamicUpstreamRegistry(gw, WithDefaultPolicy(defaultPolicy))

	if _, err := d.Add(context.Background(), cfg); err != nil {
		t.Fatalf("Add: %v", err)
	}

	stored, ok := d.Get("explicit")
	if !ok {
		t.Fatal("expected stored config for 'explicit'")
	}
	if stored.Policy.CallTimeout != explicitPolicy.CallTimeout {
		t.Fatalf("expected explicit CallTimeout %v, got %v", explicitPolicy.CallTimeout, stored.Policy.CallTimeout)
	}
}

// TestDynamicUpstreamRegistry_Hooks verifies OnAdd and OnRemove callbacks fire.
func TestDynamicUpstreamRegistry_Hooks(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "hooked", echoTool("tool1"), echoTool("tool2"))
	gw, _ := NewGateway()
	defer gw.Close()

	var addedName string
	var addedCount int
	var removedName string

	d := NewDynamicUpstreamRegistry(gw, WithDynamicHooks(DynamicHooks{
		OnAdd: func(name string, toolCount int) {
			addedName = name
			addedCount = toolCount
		},
		OnRemove: func(name string) {
			removedName = name
		},
	}))

	count, err := d.Add(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if addedName != "hooked" {
		t.Fatalf("OnAdd: expected name 'hooked', got %q", addedName)
	}
	if addedCount != count {
		t.Fatalf("OnAdd: expected count %d, got %d", count, addedCount)
	}
	if addedCount != 2 {
		t.Fatalf("OnAdd: expected 2 tools, got %d", addedCount)
	}

	if err := d.Remove("hooked"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removedName != "hooked" {
		t.Fatalf("OnRemove: expected name 'hooked', got %q", removedName)
	}
}

// TestDynamicUpstreamRegistry_RemoveUpdatesState verifies that Remove clears the
// config so Get/List/Len reflect the deletion.
func TestDynamicUpstreamRegistry_RemoveUpdatesState(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "temporary", echoTool("thing"))
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	if _, err := d.Add(context.Background(), cfg); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if d.Len() != 1 {
		t.Fatalf("expected Len 1 after Add, got %d", d.Len())
	}

	if err := d.Remove("temporary"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if d.Len() != 0 {
		t.Fatalf("expected Len 0 after Remove, got %d", d.Len())
	}
	if _, ok := d.Get("temporary"); ok {
		t.Fatal("expected Get to return false after Remove")
	}
	if len(d.List()) != 0 {
		t.Fatalf("expected empty List after Remove, got %v", d.List())
	}
}

// TestDynamicUpstreamRegistry_ConcurrentAccess exercises List/Get/Len from multiple
// goroutines concurrently to surface data races.
func TestDynamicUpstreamRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range 50 {
				_ = d.List()
				_, _ = d.Get("none")
				_ = d.Len()
			}
		}()
	}
	wg.Wait()
}

// TestDynamicUpstreamRegistry_AddFailureDoesNotStore verifies that when the underlying
// gateway rejects the connection (bad URL), the config is NOT stored.
func TestDynamicUpstreamRegistry_AddFailureDoesNotStore(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()
	d := NewDynamicUpstreamRegistry(gw)

	cfg := UpstreamConfig{
		Name: "badurl",
		URL:  "http://127.0.0.1:1", // nothing listening here
	}

	_, err := d.Add(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected connection error for unreachable server")
	}

	// Config must not have been stored on failure.
	if d.Len() != 0 {
		t.Fatalf("expected Len 0 after failed Add, got %d", d.Len())
	}
	if _, ok := d.Get("badurl"); ok {
		t.Fatal("expected Get to return false after failed Add")
	}
}
