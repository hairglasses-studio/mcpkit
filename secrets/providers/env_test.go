package providers

import (
	"context"
	"slices"
	"testing"

	"github.com/hairglasses-studio/mcpkit/secrets"
)

func TestEnvProvider_Get(t *testing.T) {
	t.Setenv("MCPKIT_TEST_KEY", "hello_world")

	p := NewEnvProvider()
	s, err := p.Get(context.Background(), "MCPKIT_TEST_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "hello_world" {
		t.Errorf("expected hello_world, got %q", s.Value)
	}
}

func TestEnvProvider_Get_CaseInsensitive(t *testing.T) {
	t.Setenv("MCPKIT_UPPER_KEY", "uppercase_value")

	p := NewEnvProvider()
	// Provider falls back to uppercase when lowercase lookup fails
	s, err := p.Get(context.Background(), "mcpkit_upper_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "uppercase_value" {
		t.Errorf("expected uppercase_value, got %q", s.Value)
	}
}

func TestEnvProvider_Get_WithPrefix(t *testing.T) {
	t.Setenv("APP_SECRET_TOKEN", "token123")

	p := NewEnvProvider(WithPrefix("APP_"))
	s, err := p.Get(context.Background(), "SECRET_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "token123" {
		t.Errorf("expected token123, got %q", s.Value)
	}
}

func TestEnvProvider_Get_NotFound(t *testing.T) {
	p := NewEnvProvider()
	_, err := p.Get(context.Background(), "MCPKIT_DEFINITELY_NOT_SET_XYZ123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != secrets.ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestEnvProvider_List(t *testing.T) {
	t.Setenv("MYPFX_ALPHA", "1")
	t.Setenv("MYPFX_BETA", "2")
	t.Setenv("MYPFX_GAMMA", "3")

	p := NewEnvProvider(WithPrefix("MYPFX_"))
	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}

	for _, want := range []string{"ALPHA", "BETA", "GAMMA"} {
		if !found[want] {
			t.Errorf("expected key %q in List, got %v", want, keys)
		}
	}
}

func TestEnvProvider_Exists(t *testing.T) {
	t.Setenv("MCPKIT_EXISTS_KEY", "yes")

	p := NewEnvProvider()

	exists, err := p.Exists(context.Background(), "MCPKIT_EXISTS_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected Exists=true for set variable")
	}

	notExists, err := p.Exists(context.Background(), "MCPKIT_NOT_SET_XYZ999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notExists {
		t.Error("expected Exists=false for unset variable")
	}
}

func TestEnvProvider_Health(t *testing.T) {
	p := NewEnvProvider()
	h := p.Health(context.Background())
	if !h.Available {
		t.Error("expected EnvProvider health to be available=true")
	}
	if h.Name != "env" {
		t.Errorf("expected name=env, got %q", h.Name)
	}
}

func TestEnvProvider_Priority(t *testing.T) {
	p := NewEnvProvider()
	if p.Priority() != 100 {
		t.Errorf("expected default priority 100, got %d", p.Priority())
	}

	p2 := NewEnvProvider(WithEnvPriority(50))
	if p2.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", p2.Priority())
	}
}

func TestEnvProvider_IsAvailable(t *testing.T) {
	p := NewEnvProvider()
	// EnvProvider is always available (env vars are always accessible).
	if !p.IsAvailable() {
		t.Error("expected IsAvailable=true for EnvProvider")
	}
}

func TestEnvProvider_Close(t *testing.T) {
	p := NewEnvProvider()
	if err := p.Close(); err != nil {
		t.Errorf("expected nil error from Close, got %v", err)
	}
}

func TestEnvProvider_List_NoPrefix(t *testing.T) {
	// Without a prefix, List should return all env vars including ones we set.
	t.Setenv("MCPKIT_NOPREFIX_LIST_TEST", "value")

	p := NewEnvProvider()
	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := slices.Contains(keys, "MCPKIT_NOPREFIX_LIST_TEST")
	if !found {
		t.Error("expected MCPKIT_NOPREFIX_LIST_TEST in List result")
	}
}

func TestEnvProvider_Exists_WithPrefix_UpperFallback(t *testing.T) {
	t.Setenv("APP_MY_SECRET", "val")

	p := NewEnvProvider(WithPrefix("APP_"))
	// Key already has the prefix — should find it.
	exists, err := p.Exists(context.Background(), "APP_MY_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected Exists=true when key already includes prefix")
	}
}

func TestEnvProvider_Get_KeyAlreadyHasPrefix(t *testing.T) {
	t.Setenv("APP_FULL_KEY", "fullvalue")

	p := NewEnvProvider(WithPrefix("APP_"))
	// When key already starts with the prefix, it should NOT prepend it again.
	s, err := p.Get(context.Background(), "APP_FULL_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "fullvalue" {
		t.Errorf("expected fullvalue, got %q", s.Value)
	}
}
