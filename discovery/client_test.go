package discovery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// newTestClient creates a Client pointed at the given test server URL.
func newTestClient(baseURL string, ttl time.Duration) *Client {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return NewClient(ClientConfig{
		BaseURL:    baseURL,
		CacheTTL:   ttl,
		HTTPClient: &http.Client{},
	})
}

// ---- NewClient defaults ----

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(ClientConfig{})

	if c.baseURL != DefaultRegistryURL {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, DefaultRegistryURL)
	}
	if c.cacheTTL != 5*time.Minute {
		t.Errorf("cacheTTL: got %v, want 5m", c.cacheTTL)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if c.cache == nil {
		t.Error("cache map should be initialised")
	}
}

func TestNewClient_CustomConfig(t *testing.T) {
	hc := &http.Client{Timeout: 3 * time.Second}
	c := NewClient(ClientConfig{
		BaseURL:    "https://example.com",
		CacheTTL:   2 * time.Minute,
		HTTPClient: hc,
	})

	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL: got %q", c.baseURL)
	}
	if c.cacheTTL != 2*time.Minute {
		t.Errorf("cacheTTL: got %v", c.cacheTTL)
	}
	if c.httpClient != hc {
		t.Error("custom HTTPClient was not preserved")
	}
}

// ---- Search ----

func TestSearch_QueryParams(t *testing.T) {
	var capturedURL *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		writeJSON(w,SearchResult{
			Servers: []ServerMetadata{{ID: "s1", Name: "Server1"}},
			Total:   1,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	q := SearchQuery{
		Query:     "weather",
		Category:  "data",
		Transport: "sse",
		Tags:      []string{"realtime", "api"},
		Limit:     10,
		Offset:    5,
	}
	result, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(result.Servers) != 1 || result.Servers[0].ID != "s1" {
		t.Errorf("unexpected result: %+v", result)
	}

	qs := capturedURL.Query()
	assertParam(t, qs, "query", "weather")
	assertParam(t, qs, "category", "data")
	assertParam(t, qs, "transport", "sse")
	assertParam(t, qs, "limit", "10")
	assertParam(t, qs, "offset", "5")

	tags := qs["tag"]
	if !containsAll(tags, "realtime", "api") {
		t.Errorf("tag params: got %v, want [realtime api]", tags)
	}
}

func TestSearch_EmptyQuery_NoQueryString(t *testing.T) {
	var capturedURL *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		writeJSON(w,SearchResult{})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	_, err := c.Search(context.Background(), SearchQuery{})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// With an empty query, no query-string params should appear.
	if capturedURL.RawQuery != "" {
		t.Errorf("expected empty query string, got %q", capturedURL.RawQuery)
	}
}

func TestSearch_ResponseDecoded(t *testing.T) {
	want := SearchResult{
		Servers: []ServerMetadata{
			{ID: "abc", Name: "My Server", Version: "1.0.0"},
		},
		Total:  1,
		Limit:  20,
		Offset: 0,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,want)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	got, err := c.Search(context.Background(), SearchQuery{Query: "my"})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if got.Total != want.Total || len(got.Servers) != 1 || got.Servers[0].ID != "abc" {
		t.Errorf("decoded result mismatch: got %+v", got)
	}
}

func TestSearch_Caching_SecondCallHitsCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w,SearchResult{Total: calls}) // total changes each call
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	q := SearchQuery{Query: "cached"}

	first, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("first Search error: %v", err)
	}
	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("second Search error: %v", err)
	}

	if calls != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls)
	}
	if first.Total != second.Total {
		t.Errorf("cached result should equal first result")
	}
}

func TestSearch_CacheExpiry_AfterTTL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w,SearchResult{Total: calls})
	}))
	defer srv.Close()

	// Use a very short TTL so we can expire it quickly.
	c := newTestClient(srv.URL, 50*time.Millisecond)
	q := SearchQuery{Query: "expiry"}

	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("first Search error: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(100 * time.Millisecond)

	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("second Search error: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 HTTP calls after TTL expiry, got %d", calls)
	}
	if second.Total != 2 {
		t.Errorf("expected refreshed Total=2, got %d", second.Total)
	}
}

func TestSearch_CacheInvalidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w,SearchResult{Total: calls})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	q := SearchQuery{Query: "invalidate"}

	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("first Search error: %v", err)
	}

	c.InvalidateCache()

	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("second Search error: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 HTTP calls after InvalidateCache, got %d", calls)
	}
	if second.Total != 2 {
		t.Errorf("expected refreshed Total=2, got %d", second.Total)
	}
}

// ---- Get ----

func TestGet_CorrectPath(t *testing.T) {
	var capturedPath string
	want := ServerMetadata{ID: "srv-42", Name: "Answer Server", Version: "2.0.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		writeJSON(w,want)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	got, err := c.Get(context.Background(), "srv-42")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	if capturedPath != "/v1/servers/srv-42" {
		t.Errorf("path: got %q, want %q", capturedPath, "/v1/servers/srv-42")
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("decoded mismatch: got %+v", got)
	}
}

func TestGet_IDURLEncoded(t *testing.T) {
	var capturedRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RequestURI preserves the raw (percent-encoded) form sent by the client.
		capturedRequestURI = r.RequestURI
		writeJSON(w,ServerMetadata{ID: "has space"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	if _, err := c.Get(context.Background(), "has space"); err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if capturedRequestURI != "/v1/servers/has%20space" {
		t.Errorf("expected URL-encoded request URI, got %q", capturedRequestURI)
	}
}

// ---- List ----

func TestList_CorrectPath(t *testing.T) {
	var capturedURL *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		writeJSON(w,SearchResult{Total: 5})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	got, err := c.List(context.Background(), 20, 40)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if got.Total != 5 {
		t.Errorf("Total: got %d, want 5", got.Total)
	}

	if capturedURL.Path != "/v1/servers" {
		t.Errorf("path: got %q, want /v1/servers", capturedURL.Path)
	}
	assertParam(t, capturedURL.Query(), "limit", "20")
	assertParam(t, capturedURL.Query(), "offset", "40")
}

// ---- mapStatusError ----

func TestMapStatusError_2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		if err := mapStatusError(code, "http://example.com"); err != nil {
			t.Errorf("status %d: expected nil, got %v", code, err)
		}
	}
}

func TestMapStatusError_404(t *testing.T) {
	err := mapStatusError(http.StatusNotFound, "http://example.com/foo")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMapStatusError_401(t *testing.T) {
	err := mapStatusError(http.StatusUnauthorized, "http://example.com")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestMapStatusError_403(t *testing.T) {
	// 403 Forbidden is also mapped to ErrUnauthorized.
	err := mapStatusError(http.StatusForbidden, "http://example.com")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for 403, got %v", err)
	}
}

func TestMapStatusError_409(t *testing.T) {
	err := mapStatusError(http.StatusConflict, "http://example.com")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestMapStatusError_429(t *testing.T) {
	err := mapStatusError(http.StatusTooManyRequests, "http://example.com")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestMapStatusError_500(t *testing.T) {
	err := mapStatusError(http.StatusInternalServerError, "http://example.com")
	if !errors.Is(err, ErrRegistryError) {
		t.Errorf("expected ErrRegistryError, got %v", err)
	}
}

func TestMapStatusError_503(t *testing.T) {
	err := mapStatusError(http.StatusServiceUnavailable, "http://example.com")
	if !errors.Is(err, ErrRegistryError) {
		t.Errorf("expected ErrRegistryError for 503, got %v", err)
	}
}

func TestMapStatusError_UnexpectedCode(t *testing.T) {
	// e.g. 400 Bad Request — not a sentinel, just a generic error
	err := mapStatusError(http.StatusBadRequest, "http://example.com")
	if err == nil {
		t.Fatal("expected non-nil error for 400")
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrConflict) || errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrRegistryError) {
		t.Errorf("400 should not match any sentinel, got %v", err)
	}
}

// ---- Error propagation via httptest ----

func TestSearch_HTTPError_ReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	_, err := c.Search(context.Background(), SearchQuery{Query: "fail"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRegistryError) {
		t.Errorf("expected ErrRegistryError, got %v", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, 5*time.Minute)
	_, err := c.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- helpers ----

func assertParam(t *testing.T, qs url.Values, key, want string) {
	t.Helper()
	if got := qs.Get(key); got != want {
		t.Errorf("query param %q: got %q, want %q", key, got, want)
	}
}

func containsAll(slice []string, values ...string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	for _, v := range values {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}
