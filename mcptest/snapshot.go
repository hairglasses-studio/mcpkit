package mcptest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// snapshotConfig holds options for AssertSnapshot.
type snapshotConfig struct {
	dir              string
	ignoreTimestamps bool
}

// SnapshotOption configures AssertSnapshot.
type SnapshotOption func(*snapshotConfig)

// WithSnapshotDir sets the directory where golden files are stored.
// Defaults to "testdata/snapshots" relative to the test's working directory.
func WithSnapshotDir(dir string) SnapshotOption {
	return func(c *snapshotConfig) {
		c.dir = dir
	}
}

// WithIgnoreTimestamps strips common timestamp fields before comparison.
// Removed fields: "timestamp", "time", "created_at", "updated_at".
func WithIgnoreTimestamps() SnapshotOption {
	return func(c *snapshotConfig) {
		c.ignoreTimestamps = true
	}
}

// timestampFields are the field names stripped when WithIgnoreTimestamps is used.
var timestampFields = []string{"timestamp", "time", "created_at", "updated_at"}

// AssertSnapshot compares result against a golden file named <name>.golden.json.
//
// When the file does not exist it is created (new snapshot mode).
// Set the environment variable UPDATE_SNAPSHOTS=1 to overwrite existing golden files.
func AssertSnapshot(t testing.TB, name string, result *registry.CallToolResult, opts ...SnapshotOption) {
	t.Helper()

	cfg := &snapshotConfig{
		dir: filepath.Join("testdata", "snapshots"),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	path := filepath.Join(cfg.dir, name+".golden.json")

	// Normalise the result to a comparable map
	normalised := normaliseResult(result, cfg)

	update := os.Getenv("UPDATE_SNAPSHOTS") == "1"

	if _, err := os.Stat(path); os.IsNotExist(err) || update {
		// Create or update the golden file
		if mkErr := os.MkdirAll(cfg.dir, 0o755); mkErr != nil {
			t.Fatalf("snapshot: create directory %s: %v", cfg.dir, mkErr)
		}
		data, marshalErr := json.MarshalIndent(normalised, "", "  ")
		if marshalErr != nil {
			t.Fatalf("snapshot: marshal result: %v", marshalErr)
		}
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			t.Fatalf("snapshot: write golden file %s: %v", path, writeErr)
		}
		if update {
			t.Logf("snapshot: updated golden file %s", path)
		} else {
			t.Logf("snapshot: created golden file %s", path)
		}
		return
	}

	// Read and compare against existing golden file
	goldenData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("snapshot: read golden file %s: %v", path, err)
	}

	var golden interface{}
	if err := json.Unmarshal(goldenData, &golden); err != nil {
		t.Fatalf("snapshot: unmarshal golden file %s: %v", path, err)
	}

	gotJSON, err := json.Marshal(normalised)
	if err != nil {
		t.Fatalf("snapshot: marshal normalised result: %v", err)
	}
	wantJSON, err := json.Marshal(golden)
	if err != nil {
		t.Fatalf("snapshot: marshal golden: %v", err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Errorf("snapshot %q mismatch\ngot:  %s\nwant: %s\n\nRun with UPDATE_SNAPSHOTS=1 to update.", name, gotJSON, wantJSON)
	}
}

// normaliseResult converts a CallToolResult to a comparable representation,
// applying any configured normalisation (e.g., timestamp stripping).
func normaliseResult(result *registry.CallToolResult, cfg *snapshotConfig) interface{} {
	if result == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	if cfg.ignoreTimestamps {
		for _, field := range timestampFields {
			delete(m, field)
		}
		// Also strip from nested structured content
		if sc, ok := m["structuredContent"]; ok {
			if scMap, ok := sc.(map[string]interface{}); ok {
				for _, field := range timestampFields {
					delete(scMap, field)
				}
			}
		}
	}

	return m
}
