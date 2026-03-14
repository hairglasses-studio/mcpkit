package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testValidator(token string) (string, error) {
	if token == "valid-token" {
		return "user-123", nil
	}
	return "", fmt.Errorf("invalid token")
}

func TestMiddleware_ValidToken(t *testing.T) {
	handler := Middleware(testValidator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub := Subject(r.Context())
		if sub != "user-123" {
			t.Errorf("subject = %q, want %q", sub, "user-123")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_NoToken(t *testing.T) {
	handler := Middleware(testValidator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate header")
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	handler := Middleware(testValidator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_NonBearerScheme(t *testing.T) {
	handler := Middleware(testValidator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSubject_NotInContext(t *testing.T) {
	if s := Subject(context.Background()); s != "" {
		t.Errorf("Subject should be empty, got %q", s)
	}
}

func TestMetadataHandler(t *testing.T) {
	meta := ProtectedResourceMetadata{
		Resource:             "https://example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
		ScopesSupported:      []string{"tools:read", "tools:write"},
	}

	handler := MetadataHandler(meta)
	req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestConfig_NewMetadata(t *testing.T) {
	cfg := Config{
		Resource:             "https://example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
		Scopes:               []string{"read"},
	}
	meta := cfg.NewMetadata()
	if meta.Resource != cfg.Resource {
		t.Errorf("resource mismatch")
	}
	if len(meta.BearerMethodsSupported) != 1 || meta.BearerMethodsSupported[0] != "header" {
		t.Error("bearer methods should be [header]")
	}
}
