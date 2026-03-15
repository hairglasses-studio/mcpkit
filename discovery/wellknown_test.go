//go:build !official_sdk

package discovery

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestServerCardHandler_GET(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&stubModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "greet", Description: "Say hello"}},
		},
	})

	h := ServerCardHandler(MetadataConfig{
		Name:        "test-server",
		Description: "A test server",
		Version:     "1.0.0",
		Tools:       reg,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q, want public, max-age=300", cc)
	}

	var card ServerCard
	if err := json.NewDecoder(rr.Body).Decode(&card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if card.Name != "test-server" {
		t.Errorf("Name = %q, want test-server", card.Name)
	}
	if card.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
	if len(card.Tools) != 1 || card.Tools[0].Name != "greet" {
		t.Errorf("Tools = %+v, want [{greet ...}]", card.Tools)
	}
}

func TestServerCardHandler_HEAD(t *testing.T) {
	t.Parallel()

	h := ServerCardHandler(MetadataConfig{Name: "test"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if len(body) != 0 {
		t.Errorf("HEAD should return empty body, got %d bytes", len(body))
	}
}

func TestServerCardHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := ServerCardHandler(MetadataConfig{Name: "test"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
	if allow := rr.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow = %q, want 'GET, HEAD'", allow)
	}
}

func TestServerCardHandler_IncludesVersion(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&stubModule{
		name: "versioned",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "calc", Description: "Calculate"},
				Version: "2.1.0",
			},
		},
	})

	h := ServerCardHandler(MetadataConfig{
		Name:  "versioned-server",
		Tools: reg,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	var card ServerCard
	if err := json.NewDecoder(rr.Body).Decode(&card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(card.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(card.Tools))
	}
	if card.Tools[0].Version != "2.1.0" {
		t.Errorf("tool version = %q, want 2.1.0", card.Tools[0].Version)
	}
}

func TestStaticServerCardHandler(t *testing.T) {
	t.Parallel()

	meta := ServerMetadata{
		Name:        "static-server",
		Description: "Pre-built",
		Version:     "3.0.0",
		Tools: []ToolSummary{
			{Name: "ping", Description: "Pong"},
		},
	}

	h := StaticServerCardHandler(meta)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var card ServerCard
	if err := json.NewDecoder(rr.Body).Decode(&card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if card.Name != "static-server" {
		t.Errorf("Name = %q, want static-server", card.Name)
	}
	if card.Version != "3.0.0" {
		t.Errorf("Version = %q, want 3.0.0", card.Version)
	}
	if len(card.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(card.Tools))
	}

	// Static handler: second request should return same GeneratedAt
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr2, req2)

	var card2 ServerCard
	json.NewDecoder(rr2.Body).Decode(&card2)
	if !card.GeneratedAt.Equal(card2.GeneratedAt) {
		t.Error("StaticServerCardHandler should return same GeneratedAt on every request")
	}
}

// stubModule implements registry.ToolModule for tests.
type stubModule struct {
	name  string
	tools []registry.ToolDefinition
}

func (m *stubModule) Name() string                     { return m.name }
func (m *stubModule) Description() string              { return m.name + " module" }
func (m *stubModule) Tools() []registry.ToolDefinition { return m.tools }
