package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetadataDiscovery_OAuthEndpoint(t *testing.T) {
	meta := AuthServerMetadata{
		Issuer:                        "https://auth.example.com",
		AuthorizationEndpoint:         "https://auth.example.com/authorize",
		TokenEndpoint:                 "https://auth.example.com/token",
		JWKSURI:                       "https://auth.example.com/.well-known/jwks.json",
		ResponseTypesSupported:        []string{"code"},
		GrantTypesSupported:           []string{"authorization_code"},
		CodeChallengeMethodsSupported: []string{"S256"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := NewMetadataDiscovery(DiscoveryConfig{})
	got, err := d.Discover(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got.Issuer != meta.Issuer {
		t.Errorf("issuer = %q, want %q", got.Issuer, meta.Issuer)
	}
	if got.JWKSURI != meta.JWKSURI {
		t.Errorf("jwks_uri = %q, want %q", got.JWKSURI, meta.JWKSURI)
	}
	if !got.SupportsPKCE() {
		t.Error("expected PKCE support")
	}
	if !got.SupportsGrant("authorization_code") {
		t.Error("expected authorization_code grant support")
	}
}

func TestMetadataDiscovery_OpenIDFallback(t *testing.T) {
	meta := AuthServerMetadata{
		Issuer:                 "https://auth.example.com",
		AuthorizationEndpoint:  "https://auth.example.com/authorize",
		TokenEndpoint:          "https://auth.example.com/token",
		JWKSURI:                "https://auth.example.com/jwks",
		ResponseTypesSupported: []string{"code"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := NewMetadataDiscovery(DiscoveryConfig{})
	got, err := d.Discover(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Discover with OIDC fallback: %v", err)
	}

	if got.Issuer != meta.Issuer {
		t.Errorf("issuer = %q, want %q", got.Issuer, meta.Issuer)
	}
}

func TestMetadataDiscovery_Cache(t *testing.T) {
	fetchCount := 0
	meta := AuthServerMetadata{
		Issuer:                 "https://auth.example.com",
		AuthorizationEndpoint:  "https://auth.example.com/authorize",
		TokenEndpoint:          "https://auth.example.com/token",
		ResponseTypesSupported: []string{"code"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	d := NewMetadataDiscovery(DiscoveryConfig{})

	for i := 0; i < 3; i++ {
		_, err := d.Discover(context.Background(), srv.URL)
		if err != nil {
			t.Fatalf("Discover %d: %v", i, err)
		}
	}

	if fetchCount != 1 {
		t.Errorf("fetched %d times, expected 1 (cached)", fetchCount)
	}
}

func TestMetadataDiscovery_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	d := NewMetadataDiscovery(DiscoveryConfig{})
	_, err := d.Discover(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

func TestMetadataDiscovery_MissingIssuer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"authorization_endpoint": "https://auth.example.com/authorize",
		})
	}))
	defer srv.Close()

	d := NewMetadataDiscovery(DiscoveryConfig{})
	_, err := d.Discover(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for missing issuer")
	}
}

func TestAuthServerMetadata_SupportsPKCE(t *testing.T) {
	tests := []struct {
		methods []string
		want    bool
	}{
		{nil, false},
		{[]string{"plain"}, false},
		{[]string{"S256"}, true},
		{[]string{"plain", "S256"}, true},
	}
	for _, tt := range tests {
		meta := AuthServerMetadata{CodeChallengeMethodsSupported: tt.methods}
		if got := meta.SupportsPKCE(); got != tt.want {
			t.Errorf("SupportsPKCE(%v) = %v, want %v", tt.methods, got, tt.want)
		}
	}
}

func TestDiscoverJWKSURL(t *testing.T) {
	meta := AuthServerMetadata{
		Issuer:                 "https://auth.example.com",
		JWKSURI:                "https://auth.example.com/jwks",
		ResponseTypesSupported: []string{"code"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	url, err := DiscoverJWKSURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverJWKSURL: %v", err)
	}
	if url != meta.JWKSURI {
		t.Errorf("url = %q, want %q", url, meta.JWKSURI)
	}
}
