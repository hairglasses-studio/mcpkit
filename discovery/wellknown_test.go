package discovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if card.ToolCount != 1 {
		t.Errorf("ToolCount = %d, want 1", card.ToolCount)
	}
}

// TestServerCardHandler_ToolCount_MatchesLiveRegistry pins down that
// tool_count is derived from the live registry at generation time (not
// len(Tools), and not a hand-maintained constant) so fleetinventory's
// declared-gap dimension can trust the value on every request.
func TestServerCardHandler_ToolCount_MatchesLiveRegistry(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&stubModule{
		name: "multi",
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "one", Description: "One"}},
			{Tool: registry.Tool{Name: "two", Description: "Two"}},
			{Tool: registry.Tool{Name: "three", Description: "Three"}},
		},
	})

	h := ServerCardHandler(MetadataConfig{Name: "counted-server", Tools: reg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	h.ServeHTTP(rr, req)

	// Assert against the raw JSON, not just the decoded struct, since this
	// is the exact shape fleetinventory's declaredGap() unmarshals
	// (a flat top-level "tool_count" key).
	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	got, ok := raw["tool_count"]
	if !ok {
		t.Fatal("raw JSON missing top-level \"tool_count\" key")
	}
	if got != float64(3) {
		t.Errorf("raw tool_count = %v, want 3", got)
	}

	// Register a 4th tool and confirm a fresh request reflects the live
	// registry rather than a value cached at handler-construction time.
	reg.RegisterModule(&stubModule{
		name:  "extra",
		tools: []registry.ToolDefinition{{Tool: registry.Tool{Name: "four", Description: "Four"}}},
	})

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	h.ServeHTTP(rr2, req2)

	var card2 ServerCard
	if err := json.NewDecoder(rr2.Body).Decode(&card2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if card2.ToolCount != 4 {
		t.Errorf("ToolCount after registering a 4th tool = %d, want 4", card2.ToolCount)
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

func TestWriteFile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, ".well-known", "mcp.json")

	meta := ServerMetadata{
		Name:        "roundtrip-server",
		Description: "Test round-trip write",
		Version:     "1.0.0",
		Homepage:    "https://example.com",
		License:     "MIT",
		Categories:  []string{"developer-tools", "testing"},
		Install: &InstallInfo{
			Go: "go install github.com/example/roundtrip@latest",
		},
		Tags:  []string{"go", "mcp"},
		Tools: []ToolSummary{{Name: "ping", Description: "Pong"}},
	}
	card := ServerCard{
		ServerMetadata: meta,
		GeneratedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := WriteFile(dest, card); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got ServerCard
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Name != "roundtrip-server" {
		t.Errorf("Name = %q, want roundtrip-server", got.Name)
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", got.Version)
	}
	if got.Homepage != "https://example.com" {
		t.Errorf("Homepage = %q, want https://example.com", got.Homepage)
	}
	if got.License != "MIT" {
		t.Errorf("License = %q, want MIT", got.License)
	}
	if len(got.Categories) != 2 || got.Categories[0] != "developer-tools" {
		t.Errorf("Categories = %v, want [developer-tools testing]", got.Categories)
	}
	if got.Install == nil || got.Install.Go != "go install github.com/example/roundtrip@latest" {
		t.Errorf("Install.Go = %q, unexpected", got.Install)
	}
	if got.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "ping" {
		t.Errorf("Tools = %v, want [{ping Pong}]", got.Tools)
	}
}

// TestHandleContractWrite_ToolCount confirms the --contract-write path
// (used by CI/repo-generation scripts to collect .well-known/mcp.json from
// binaries) writes a live-registry-derived tool_count, matching the shape
// fleetinventory.declaredGap expects.
func TestHandleContractWrite_ToolCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, ".well-known", "mcp.json")

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&stubModule{
		name: "contract",
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "a", Description: "A"}},
			{Tool: registry.Tool{Name: "b", Description: "B"}},
		},
	})

	err := HandleContractWrite(dest, MetadataConfig{Name: "contract-count-server", Tools: reg})
	if !errors.Is(err, ErrContractWritten) {
		t.Fatalf("expected ErrContractWritten, got %v", err)
	}

	data, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("file not written: %v", readErr)
	}

	var doc struct {
		ToolCount int `json:"tool_count"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc.ToolCount != 2 {
		t.Errorf("tool_count = %d, want 2", doc.ToolCount)
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Nested path that doesn't yet exist.
	dest := filepath.Join(dir, "a", "b", "c", "mcp.json")

	card := ServerCard{
		ServerMetadata: ServerMetadata{Name: "deep-server"},
		GeneratedAt:    time.Now().UTC(),
	}
	if err := WriteFile(dest, card); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("file not found after WriteFile: %v", err)
	}
}

func TestWriteFile_Atomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "mcp.json")

	// Write an initial file.
	card1 := ServerCard{
		ServerMetadata: ServerMetadata{Name: "v1"},
		GeneratedAt:    time.Now().UTC(),
	}
	if err := WriteFile(dest, card1); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}

	// Overwrite atomically.
	card2 := ServerCard{
		ServerMetadata: ServerMetadata{Name: "v2"},
		GeneratedAt:    time.Now().UTC(),
	}
	if err := WriteFile(dest, card2); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	data, _ := os.ReadFile(dest)
	var got ServerCard
	json.Unmarshal(data, &got)
	if got.Name != "v2" {
		t.Errorf("Name = %q, want v2", got.Name)
	}

	// No leftover temp files in the dir.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("unexpected files after atomic overwrite: %v", entries)
	}
}

func TestHandleContractWrite_Empty(t *testing.T) {
	t.Parallel()

	// Empty dest → no-op, no error.
	if err := HandleContractWrite("", MetadataConfig{Name: "noop"}); err != nil {
		t.Errorf("expected nil for empty dest, got %v", err)
	}
}

func TestHandleContractWrite_WritesAndReturnsErrContractWritten(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, ".well-known", "mcp.json")

	cfg := MetadataConfig{
		Name:       "contract-server",
		Version:    "2.0.0",
		License:    "Apache-2.0",
		Categories: []string{"developer-tools"},
		Install:    &InstallInfo{Go: "go install example.com/contract-server@latest"},
	}

	err := HandleContractWrite(dest, cfg)
	if err == nil {
		t.Fatal("expected ErrContractWritten, got nil")
	}
	if !errors.Is(err, ErrContractWritten) {
		t.Errorf("expected ErrContractWritten, got %v", err)
	}

	data, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("file not written: %v", readErr)
	}

	var got ServerCard
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != "contract-server" {
		t.Errorf("Name = %q, want contract-server", got.Name)
	}
	if got.License != "Apache-2.0" {
		t.Errorf("License = %q, want Apache-2.0", got.License)
	}
	if len(got.Categories) != 1 || got.Categories[0] != "developer-tools" {
		t.Errorf("Categories = %v", got.Categories)
	}
	if got.Install == nil || got.Install.Go != "go install example.com/contract-server@latest" {
		t.Errorf("Install.Go unexpected: %v", got.Install)
	}
}

func TestServerCardHandler_IncludesCategories(t *testing.T) {
	t.Parallel()

	h := ServerCardHandler(MetadataConfig{
		Name:       "cats-server",
		Version:    "1.0.0",
		Categories: []string{"linux", "developer-tools"},
		License:    "MIT",
		Homepage:   "https://example.com/cats",
		Install:    &InstallInfo{Go: "go install example.com/cats@latest"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	h.ServeHTTP(rr, req)

	var card ServerCard
	if err := json.NewDecoder(rr.Body).Decode(&card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(card.Categories) != 2 {
		t.Errorf("Categories len = %d, want 2", len(card.Categories))
	}
	if card.License != "MIT" {
		t.Errorf("License = %q, want MIT", card.License)
	}
	if card.Homepage != "https://example.com/cats" {
		t.Errorf("Homepage = %q, want https://example.com/cats", card.Homepage)
	}
	if card.Install == nil || card.Install.Go == "" {
		t.Errorf("Install.Go missing")
	}
}

func TestInstallInfo_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	info := InstallInfo{
		Go:     "go install example.com/server@latest",
		NPM:    "npm install -g @example/server",
		PyPI:   "pip install example-server",
		Brew:   "brew install example/tap/server",
		Docker: "docker pull example/server:latest",
		Binary: "https://example.com/releases/latest",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got InstallInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != info {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, info)
	}
}

func TestInstallInfo_Empty_OmittedFromJSON(t *testing.T) {
	t.Parallel()

	meta := ServerMetadata{
		Name:    "no-install-server",
		Version: "1.0.0",
		// Install is nil — should not appear in JSON.
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(data, []byte(`"install"`)) {
		t.Errorf("unexpected 'install' key in JSON output when Install is nil: %s", data)
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
