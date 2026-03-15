package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetadataHandler_JSONBody(t *testing.T) {
	meta := ProtectedResourceMetadata{
		Resource:             "https://api.example.com",
		AuthorizationServers: []string{"https://auth.example.com"},
		ScopesSupported:      []string{"read", "write"},
	}

	handler := MetadataHandler(meta)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("MetadataHandler status = %d, want 200", rec.Code)
	}

	var got ProtectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("MetadataHandler response not valid JSON: %v", err)
	}

	if got.Resource != meta.Resource {
		t.Errorf("Resource = %q, want %q", got.Resource, meta.Resource)
	}
	if len(got.AuthorizationServers) != 1 || got.AuthorizationServers[0] != meta.AuthorizationServers[0] {
		t.Errorf("AuthorizationServers = %v, want %v", got.AuthorizationServers, meta.AuthorizationServers)
	}
	if len(got.ScopesSupported) != 2 {
		t.Errorf("ScopesSupported = %v, want %v", got.ScopesSupported, meta.ScopesSupported)
	}
}

func TestMetadataHandler_EmptyMetadata(t *testing.T) {
	meta := ProtectedResourceMetadata{}

	handler := MetadataHandler(meta)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("MetadataHandler status = %d, want 200", rec.Code)
	}

	var got ProtectedResourceMetadata
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("MetadataHandler empty response not valid JSON: %v", err)
	}

	// Empty metadata should have zero-value fields
	if got.Resource != "" {
		t.Errorf("Resource = %q, want empty string", got.Resource)
	}
	if len(got.AuthorizationServers) != 0 {
		t.Errorf("AuthorizationServers = %v, want empty slice", got.AuthorizationServers)
	}
}

func TestMetadataHandler_ContentType(t *testing.T) {
	handler := MetadataHandler(ProtectedResourceMetadata{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
