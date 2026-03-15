package auth

import (
	"testing"
)

func TestNewMetadata_AllFields(t *testing.T) {
	cfg := Config{
		Resource:             "https://api.example.com",
		AuthorizationServers: []string{"https://auth.example.com", "https://auth2.example.com"},
		Scopes:               []string{"read", "write", "admin"},
	}

	meta := cfg.NewMetadata()

	if meta.Resource != cfg.Resource {
		t.Errorf("Resource = %q, want %q", meta.Resource, cfg.Resource)
	}
	if len(meta.AuthorizationServers) != len(cfg.AuthorizationServers) {
		t.Errorf("AuthorizationServers length = %d, want %d", len(meta.AuthorizationServers), len(cfg.AuthorizationServers))
	}
	for i, s := range cfg.AuthorizationServers {
		if meta.AuthorizationServers[i] != s {
			t.Errorf("AuthorizationServers[%d] = %q, want %q", i, meta.AuthorizationServers[i], s)
		}
	}
	if len(meta.ScopesSupported) != len(cfg.Scopes) {
		t.Errorf("ScopesSupported length = %d, want %d", len(meta.ScopesSupported), len(cfg.Scopes))
	}
	for i, s := range cfg.Scopes {
		if meta.ScopesSupported[i] != s {
			t.Errorf("ScopesSupported[%d] = %q, want %q", i, meta.ScopesSupported[i], s)
		}
	}
}

func TestNewMetadata_EmptyOptionals(t *testing.T) {
	cfg := Config{
		Resource:             "https://api.example.com",
		AuthorizationServers: []string{"https://auth.example.com"},
		// Scopes intentionally omitted
	}

	meta := cfg.NewMetadata()

	if meta.Resource != cfg.Resource {
		t.Errorf("Resource = %q, want %q", meta.Resource, cfg.Resource)
	}
	if len(meta.ScopesSupported) != 0 {
		t.Errorf("ScopesSupported = %v, want empty", meta.ScopesSupported)
	}
}

func TestNewMetadata_BearerMethodsSupported(t *testing.T) {
	cfg := Config{
		Resource:             "https://api.example.com",
		AuthorizationServers: []string{"https://auth.example.com"},
	}

	meta := cfg.NewMetadata()

	containsHeader := false
	for _, m := range meta.BearerMethodsSupported {
		if m == "header" {
			containsHeader = true
			break
		}
	}
	if !containsHeader {
		t.Errorf("BearerMethodsSupported = %v, want it to contain \"header\"", meta.BearerMethodsSupported)
	}
}
