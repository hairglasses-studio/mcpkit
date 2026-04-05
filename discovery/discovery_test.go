package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// sampleServer returns a minimal ServerMetadata for use in tests.
func sampleServer(id, name string) ServerMetadata {
	return ServerMetadata{
		ID:          id,
		Name:        name,
		Description: "A test server",
		Version:     "1.0.0",
	}
}

// sampleResult wraps a single server in a SearchResult.
func sampleResult(id, name string) SearchResult {
	return SearchResult{
		Servers: []ServerMetadata{sampleServer(id, name)},
		Total:   1,
		Limit:   10,
		Offset:  0,
	}
}

// writeJSON encodes v as JSON and writes it to w with 200 OK.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Client tests ---

func TestClient_Search(t *testing.T) {
	t.Parallel()
	want := sampleResult("srv-1", "My Server")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/servers" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, want)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	got, err := c.Search(context.Background(), SearchQuery{Query: "test", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(got.Servers))
	}
	if got.Servers[0].ID != "srv-1" {
		t.Errorf("server ID = %q, want %q", got.Servers[0].ID, "srv-1")
	}
}

func TestClient_Search_Cache(t *testing.T) {
	t.Parallel()
	var hitCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hitCount, 1)
		writeJSON(w, sampleResult("srv-1", "Cached Server"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, CacheTTL: time.Minute})
	q := SearchQuery{Query: "cache-test", Limit: 5}

	for i := range 2 {
		_, err := c.Search(context.Background(), q)
		if err != nil {
			t.Fatalf("Search %d: %v", i, err)
		}
	}

	if n := atomic.LoadInt64(&hitCount); n != 1 {
		t.Errorf("backend hit %d times, want 1 (second call should be cached)", n)
	}
}

func TestClient_Search_CacheExpiry(t *testing.T) {
	t.Parallel()
	var hitCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hitCount, 1)
		writeJSON(w, sampleResult("srv-1", "Expiry Server"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, CacheTTL: 10 * time.Millisecond})
	q := SearchQuery{Query: "expiry-test"}

	// First call — populates cache.
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("Search 1: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(20 * time.Millisecond)

	// Second call — cache expired, should re-fetch.
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("Search 2: %v", err)
	}

	if n := atomic.LoadInt64(&hitCount); n != 2 {
		t.Errorf("backend hit %d times, want 2 (cache should have expired)", n)
	}
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	want := sampleServer("srv-42", "Get Server")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/servers/srv-42" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, want)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	got, err := c.Get(context.Background(), "srv-42")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "srv-42" {
		t.Errorf("ID = %q, want %q", got.ID, "srv-42")
	}
	if got.Name != "Get Server" {
		t.Errorf("Name = %q, want %q", got.Name, "Get Server")
	}
}

func TestClient_Get_NotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	_, err := c.Get(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestClient_Get_Unauthorized(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	_, err := c.Get(context.Background(), "protected")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("error = %v, want ErrUnauthorized", err)
	}
}

func TestClient_Get_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	_, err := c.Get(context.Background(), "srv-99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("error = %v, want ErrRateLimited", err)
	}
}

func TestClient_InvalidateCache(t *testing.T) {
	t.Parallel()
	var hitCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hitCount, 1)
		writeJSON(w, sampleResult("srv-1", "Invalidate Server"))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, CacheTTL: time.Hour})
	q := SearchQuery{Query: "invalidate-test"}

	// Populate cache.
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("Search 1: %v", err)
	}

	// Invalidate.
	c.InvalidateCache()

	// Second call should re-fetch.
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatalf("Search 2: %v", err)
	}

	if n := atomic.LoadInt64(&hitCount); n != 2 {
		t.Errorf("backend hit %d times, want 2 (after invalidation)", n)
	}
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	want := SearchResult{
		Servers: []ServerMetadata{
			sampleServer("a", "Server A"),
			sampleServer("b", "Server B"),
		},
		Total:  50,
		Limit:  2,
		Offset: 10,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/servers" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("offset") != "10" {
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}
		writeJSON(w, want)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	got, err := c.List(context.Background(), 2, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Total != 50 {
		t.Errorf("Total = %d, want 50", got.Total)
	}
	if len(got.Servers) != 2 {
		t.Errorf("got %d servers, want 2", len(got.Servers))
	}
}

// --- Publisher tests ---

func TestPublisher_Register(t *testing.T) {
	t.Parallel()
	var gotBody ServerMetadata

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/servers" {
			http.Error(w, "bad route", http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		gotBody.ID = "new-id"
		writeJSON(w, gotBody)
	}))
	defer srv.Close()

	p, err := NewPublisher(PublisherConfig{BaseURL: srv.URL, Token: "tok"})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	meta := ServerMetadata{Name: "My Server", Description: "A great server"}
	result, err := p.Register(context.Background(), meta)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.ID != "new-id" {
		t.Errorf("result.ID = %q, want %q", result.ID, "new-id")
	}
	if gotBody.Name != "My Server" {
		t.Errorf("request body Name = %q, want %q", gotBody.Name, "My Server")
	}
}

func TestPublisher_Update(t *testing.T) {
	t.Parallel()
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		gotPath = r.URL.Path
		var meta ServerMetadata
		json.NewDecoder(r.Body).Decode(&meta)
		meta.ID = "upd-id"
		writeJSON(w, meta)
	}))
	defer srv.Close()

	p, err := NewPublisher(PublisherConfig{BaseURL: srv.URL, Token: "tok"})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	_, err = p.Update(context.Background(), "upd-id", ServerMetadata{Name: "Updated"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotPath != "/v1/servers/upd-id" {
		t.Errorf("path = %q, want %q", gotPath, "/v1/servers/upd-id")
	}
}

func TestPublisher_Deregister(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p, err := NewPublisher(PublisherConfig{BaseURL: srv.URL, Token: "tok"})
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}

	if err := p.Deregister(context.Background(), "del-id"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v1/servers/del-id" {
		t.Errorf("path = %q, want %q", gotPath, "/v1/servers/del-id")
	}
}

func TestPublisher_RequiresToken(t *testing.T) {
	t.Parallel()
	_, err := NewPublisher(PublisherConfig{BaseURL: "http://example.com", Token: ""})
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

// --- MetadataFromRegistry test ---

func TestMetadataFromRegistry(t *testing.T) {
	t.Parallel()

	// Build a minimal ToolModule and register it.
	module := &testModule{
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "tool-alpha", Description: "Alpha tool"}},
			{Tool: registry.Tool{Name: "tool-beta", Description: "Beta tool"}},
		},
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(module)

	transports := []TransportInfo{{Type: "streamable-http", URL: "https://example.com/mcp"}}
	meta := MetadataFromRegistry("test-server", "A test server", reg, transports)

	if meta.Name != "test-server" {
		t.Errorf("Name = %q, want %q", meta.Name, "test-server")
	}
	if len(meta.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(meta.Tools))
	}

	// Verify tool names are present (sorted).
	if meta.Tools[0].Name != "tool-alpha" {
		t.Errorf("Tools[0].Name = %q, want %q", meta.Tools[0].Name, "tool-alpha")
	}
	if meta.Tools[1].Name != "tool-beta" {
		t.Errorf("Tools[1].Name = %q, want %q", meta.Tools[1].Name, "tool-beta")
	}
	if meta.Tools[0].Description != "Alpha tool" {
		t.Errorf("Tools[0].Description = %q, want %q", meta.Tools[0].Description, "Alpha tool")
	}
	if len(meta.Transports) != 1 || meta.Transports[0].Type != "streamable-http" {
		t.Errorf("unexpected transports: %v", meta.Transports)
	}
}

// --- SearchQuery.cacheKey test ---

func TestSearchQuery_CacheKey(t *testing.T) {
	t.Parallel()

	q1 := SearchQuery{Query: "hello", Category: "cat", Transport: "sse", Tags: []string{"a", "b"}, Limit: 10, Offset: 0}
	q2 := SearchQuery{Query: "hello", Category: "cat", Transport: "sse", Tags: []string{"b", "a"}, Limit: 10, Offset: 0}
	q3 := SearchQuery{Query: "world", Category: "cat", Transport: "sse", Tags: []string{"a", "b"}, Limit: 10, Offset: 0}

	// Same query with different tag order → same key.
	if q1.cacheKey() != q2.cacheKey() {
		t.Errorf("tag order should not affect cache key: %q != %q", q1.cacheKey(), q2.cacheKey())
	}

	// Different query → different key.
	if q1.cacheKey() == q3.cacheKey() {
		t.Errorf("different queries should produce different cache keys")
	}
}

// --- helpers ---

// testModule is a minimal ToolModule for use in tests.
type testModule struct {
	tools []registry.ToolDefinition
}

func (m *testModule) Name() string                     { return "test" }
func (m *testModule) Description() string              { return "test module" }
func (m *testModule) Tools() []registry.ToolDefinition { return m.tools }
