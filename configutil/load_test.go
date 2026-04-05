package configutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Name  string `json:"name"`
	Port  int    `json:"port"`
	Debug bool   `json:"debug"`
}

func TestLoadJSON_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"name":"app","port":9090,"debug":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadJSON[testConfig](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "app" {
		t.Errorf("Name = %q, want %q", cfg.Name, "app")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9090)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadJSON_FileNotFound(t *testing.T) {
	_, err := LoadJSON[testConfig]("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist in chain, got: %v", err)
	}
}

func TestLoadJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not valid json}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadJSON[testConfig](path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestSaveJSON_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	cfg := testConfig{Name: "test", Port: 8080, Debug: false}

	if err := SaveJSON(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !contains(string(data), `"name": "test"`) {
		t.Errorf("expected name in output, got: %s", data)
	}
	// Check trailing newline
	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}

func TestSaveJSON_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.json")
	cfg := testConfig{Name: "atomic", Port: 1234}

	if err := SaveJSON(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no temp files remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if contains(e.Name(), ".configutil-") && contains(e.Name(), ".tmp") {
			t.Errorf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestSaveJSON_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.json")
	cfg := testConfig{Name: "nested"}

	if err := SaveJSON(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestSaveJSON_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.json")
	original := testConfig{Name: "round", Port: 5555, Debug: true}

	if err := SaveJSON(path, original); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadJSON[testConfig](path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded != original {
		t.Errorf("round trip mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestLoadJSONWithDefault_Exists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.json")
	if err := os.WriteFile(path, []byte(`{"name":"fromfile","port":3000,"debug":false}`), 0644); err != nil {
		t.Fatal(err)
	}

	dflt := testConfig{Name: "default", Port: 9999}
	cfg, err := LoadJSONWithDefault(path, dflt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "fromfile" {
		t.Errorf("Name = %q, want %q", cfg.Name, "fromfile")
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want %d", cfg.Port, 3000)
	}
}

func TestLoadJSONWithDefault_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}

	dflt := testConfig{Name: "fallback"}
	_, err := LoadJSONWithDefault(path, dflt)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSaveJSON_MarshalError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marshal.json")

	// json.Marshal fails on channels
	type badType struct {
		Ch chan int `json:"ch"`
	}
	err := SaveJSON(path, badType{Ch: make(chan int)})
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !contains(err.Error(), "marshal") {
		t.Errorf("expected marshal in error, got: %v", err)
	}
}

func TestSaveJSON_ReadOnlyDir(t *testing.T) {
	// Try to save to a path in a non-writable directory
	path := "/proc/nonexistent/config.json"
	err := SaveJSON(path, testConfig{Name: "test"})
	if err == nil {
		t.Fatal("expected error writing to /proc, got nil")
	}
}

func TestSaveJSON_TempFileFailure(t *testing.T) {
	// Create a read-only directory so CreateTemp fails
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnly, 0555); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(readOnly, "config.json")
	err := SaveJSON(path, testConfig{Name: "test"})
	if err == nil {
		t.Fatal("expected error creating temp file in read-only dir, got nil")
	}
	if !contains(err.Error(), "temp file") {
		t.Errorf("expected 'temp file' in error, got: %v", err)
	}
}

func TestLoadJSONWithDefault_Missing(t *testing.T) {
	dflt := testConfig{Name: "fallback", Port: 4444, Debug: true}
	cfg, err := LoadJSONWithDefault("/nonexistent/missing.json", dflt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != dflt {
		t.Errorf("got %+v, want default %+v", cfg, dflt)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
