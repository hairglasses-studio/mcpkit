package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCredentialsTokenSourceFetchesAndCaches(t *testing.T) {
	t.Parallel()

	var tokenRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "client_credentials" {
			t.Fatalf("grant_type = %q, want client_credentials", got)
		}
		if got := r.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q, want client-id", got)
		}
		if got := r.Form.Get("scope"); got != "registry.publish registry.delete" {
			t.Fatalf("scope = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oauth-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	source, err := NewClientCredentialsTokenSource(ClientCredentialsConfig{
		TokenURL:     srv.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Scopes:       []string{"registry.publish", "registry.delete"},
	})
	if err != nil {
		t.Fatalf("NewClientCredentialsTokenSource: %v", err)
	}

	for i := 0; i < 2; i++ {
		token, err := source.Token(context.Background())
		if err != nil {
			t.Fatalf("Token: %v", err)
		}
		if token != "oauth-token" {
			t.Fatalf("token = %q, want oauth-token", token)
		}
	}
	if tokenRequests != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests)
	}
}

func TestClientCredentialsTokenSourceBasicAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client-id" || pass != "client-secret" {
			t.Fatalf("basic auth = %q/%q ok=%v", user, pass, ok)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("client_id") != "" || r.Form.Get("client_secret") != "" {
			t.Fatal("client credentials should not be in the form when UseBasicAuth is true")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "basic-token",
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	source, err := NewClientCredentialsTokenSource(ClientCredentialsConfig{
		TokenURL:     srv.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		UseBasicAuth: true,
	})
	if err != nil {
		t.Fatalf("NewClientCredentialsTokenSource: %v", err)
	}
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "basic-token" {
		t.Fatalf("token = %q, want basic-token", token)
	}
}

func TestPublisherUsesTokenSource(t *testing.T) {
	t.Parallel()

	var tokenCalls int
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "registry-oauth-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	var authHeader string
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		writeJSON(w, ServerMetadata{ID: "published", Name: "published"})
	}))
	defer registryServer.Close()

	source, err := NewClientCredentialsTokenSource(ClientCredentialsConfig{
		TokenURL:     tokenServer.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatalf("NewClientCredentialsTokenSource: %v", err)
	}
	publisher, err := NewPublisher(PublisherConfig{
		BaseURL:     registryServer.URL,
		TokenSource: source,
	})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	if _, err := publisher.Register(context.Background(), ServerMetadata{Name: "server"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if authHeader != "Bearer registry-oauth-token" {
		t.Fatalf("Authorization = %q, want OAuth bearer", authHeader)
	}
	if tokenCalls != 1 {
		t.Fatalf("token calls = %d, want 1", tokenCalls)
	}
}
