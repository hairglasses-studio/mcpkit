package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestPublisher creates a Publisher pointed at the given test server URL.
func newTestPublisher(t *testing.T, baseURL string) *Publisher {
	t.Helper()
	p, err := NewPublisher(PublisherConfig{
		BaseURL:    baseURL,
		Token:      "test-token",
		HTTPClient: &http.Client{},
	})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	return p
}

// ---- NewPublisher ----

func TestNewPublisher_MissingToken(t *testing.T) {
	_, err := NewPublisher(PublisherConfig{BaseURL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error when Token is empty")
	}
}

func TestNewPublisher_ValidToken_Defaults(t *testing.T) {
	p, err := NewPublisher(PublisherConfig{Token: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.baseURL != DefaultRegistryURL {
		t.Errorf("baseURL: got %q, want %q", p.baseURL, DefaultRegistryURL)
	}
	if p.token != "secret" {
		t.Errorf("token: got %q, want %q", p.token, "secret")
	}
	if p.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewPublisher_CustomBaseURL(t *testing.T) {
	p, err := NewPublisher(PublisherConfig{
		BaseURL: "https://my-registry.example.com",
		Token:   "tok",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.baseURL != "https://my-registry.example.com" {
		t.Errorf("baseURL: got %q", p.baseURL)
	}
}

// ---- Register ----

func TestRegister_PostBodyAndAuthHeader(t *testing.T) {
	var (
		capturedMethod string
		capturedPath   string
		capturedBody   []byte
		capturedAuth   string
	)

	want := ServerMetadata{ID: "new-id", Name: "My Server", Version: "1.0.0"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedBody, _ = io.ReadAll(r.Body)
		writeJSON(w, want)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	payload := ServerMetadata{Name: "My Server", Version: "1.0.0"}
	got, err := p.Register(context.Background(), payload)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if capturedMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", capturedMethod)
	}
	if capturedPath != "/v1/servers" {
		t.Errorf("path: got %q, want /v1/servers", capturedPath)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("auth header: got %q", capturedAuth)
	}

	// Verify the request body decoded back to the original payload.
	var sent ServerMetadata
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if sent.Name != payload.Name {
		t.Errorf("body name: got %q, want %q", sent.Name, payload.Name)
	}

	if got.ID != want.ID {
		t.Errorf("response ID: got %q, want %q", got.ID, want.ID)
	}
}

// ---- Update ----

func TestUpdate_PutRequest(t *testing.T) {
	var (
		capturedMethod string
		capturedPath   string
		capturedAuth   string
	)

	response := ServerMetadata{ID: "srv-1", Name: "Updated"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		writeJSON(w, response)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	got, err := p.Update(context.Background(), "srv-1", ServerMetadata{Name: "Updated"})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	if capturedMethod != http.MethodPut {
		t.Errorf("method: got %q, want PUT", capturedMethod)
	}
	if capturedPath != "/v1/servers/srv-1" {
		t.Errorf("path: got %q, want /v1/servers/srv-1", capturedPath)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("auth header: got %q", capturedAuth)
	}
	if got.ID != "srv-1" {
		t.Errorf("response ID: got %q", got.ID)
	}
}

func TestUpdate_IDURLEncoded(t *testing.T) {
	var capturedRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RequestURI preserves the raw (percent-encoded) form sent by the client.
		capturedRequestURI = r.RequestURI
		writeJSON(w, ServerMetadata{ID: "has space"})
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	if _, err := p.Update(context.Background(), "has space", ServerMetadata{}); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if capturedRequestURI != "/v1/servers/has%20space" {
		t.Errorf("expected URL-encoded request URI, got %q", capturedRequestURI)
	}
}

// ---- Deregister ----

func TestDeregister_DeleteRequest(t *testing.T) {
	var (
		capturedMethod string
		capturedPath   string
		capturedAuth   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	err := p.Deregister(context.Background(), "srv-del")
	if err != nil {
		t.Fatalf("Deregister error: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method: got %q, want DELETE", capturedMethod)
	}
	if capturedPath != "/v1/servers/srv-del" {
		t.Errorf("path: got %q, want /v1/servers/srv-del", capturedPath)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("auth header: got %q", capturedAuth)
	}
}

func TestDeregister_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	err := p.Deregister(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- Publish convenience wrapper ----

func TestPublish_ConvenienceWrapper(t *testing.T) {
	want := ServerMetadata{ID: "pub-1", Name: "Published"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Publish: expected POST, got %q", r.Method)
		}
		writeJSON(w, want)
	}))
	defer srv.Close()

	got, err := Publish(context.Background(), srv.URL, "tok", ServerMetadata{Name: "Published"})
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
}

func TestPublish_MissingToken_ReturnsError(t *testing.T) {
	_, err := Publish(context.Background(), "https://example.com", "", ServerMetadata{})
	if err == nil {
		t.Fatal("expected error when token is empty")
	}
}

// ---- Unpublish convenience wrapper ----

func TestUnpublish_ConvenienceWrapper(t *testing.T) {
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := Unpublish(context.Background(), srv.URL, "tok", "srv-x")
	if err != nil {
		t.Fatalf("Unpublish error: %v", err)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("method: got %q, want DELETE", capturedMethod)
	}
}

func TestUnpublish_MissingToken_ReturnsError(t *testing.T) {
	err := Unpublish(context.Background(), "https://example.com", "", "srv-x")
	if err == nil {
		t.Fatal("expected error when token is empty")
	}
}

// ---- Error status mapping ----

func TestRegister_Non2xx_MapsToSentinel(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrUnauthorized},
		{http.StatusConflict, ErrConflict},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrRegistryError},
		{http.StatusBadGateway, ErrRegistryError},
	}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			p := newTestPublisher(t, srv.URL)
			_, err := p.Register(context.Background(), ServerMetadata{Name: "x"})
			if err == nil {
				t.Fatalf("status %d: expected error, got nil", tc.status)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("status %d: got %v, want %v", tc.status, err, tc.want)
			}
		})
	}
}

func TestUpdate_Non2xx_MapsToSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	_, err := p.Update(context.Background(), "gone", ServerMetadata{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeregister_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTestPublisher(t, srv.URL)
	err := p.Deregister(context.Background(), "srv-id")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}
