//go:build !official_sdk

package mcptest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestAssertSnapshot_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	result := registry.MakeTextResult("hello snapshot")

	// Should create the golden file without failing
	AssertSnapshot(t, "new-snapshot", result, WithSnapshotDir(dir))

	// Verify golden file was created
	path := filepath.Join(dir, "new-snapshot.golden.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("golden file was not created: %v", err)
	}
}

func TestAssertSnapshot_MatchesExisting(t *testing.T) {
	dir := t.TempDir()
	result := registry.MakeTextResult("snapshot content")

	// Create the golden file
	AssertSnapshot(t, "match-test", result, WithSnapshotDir(dir))

	// Now assert the same result matches
	AssertSnapshot(t, "match-test", result, WithSnapshotDir(dir))
}

func TestAssertSnapshot_MismatchDetected(t *testing.T) {
	dir := t.TempDir()

	// Create golden file with one result
	original := registry.MakeTextResult("original content")
	AssertSnapshot(t, "mismatch-test", original, WithSnapshotDir(dir))

	// Assert a different result — should fail
	different := registry.MakeTextResult("different content")

	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	AssertSnapshot(mockT, "mismatch-test", different, WithSnapshotDir(dir))
	if !failed {
		t.Error("AssertSnapshot should have detected mismatch but did not")
	}
}

func TestAssertSnapshot_UpdateMode(t *testing.T) {
	dir := t.TempDir()

	// Create initial golden file
	original := registry.MakeTextResult("original")
	AssertSnapshot(t, "update-test", original, WithSnapshotDir(dir))

	// Set update mode and write a new result
	t.Setenv("UPDATE_SNAPSHOTS", "1")

	updated := registry.MakeTextResult("updated content")
	AssertSnapshot(t, "update-test", updated, WithSnapshotDir(dir))

	// Now match without update mode — should match the updated content
	t.Setenv("UPDATE_SNAPSHOTS", "")
	AssertSnapshot(t, "update-test", updated, WithSnapshotDir(dir))
}

func TestAssertSnapshot_WithIgnoreTimestamps(t *testing.T) {
	dir := t.TempDir()

	result := registry.MakeTextResult("time-sensitive")

	// Create snapshot
	AssertSnapshot(t, "timestamp-test", result, WithSnapshotDir(dir), WithIgnoreTimestamps())

	// Should still match with timestamps ignored
	AssertSnapshot(t, "timestamp-test", result, WithSnapshotDir(dir), WithIgnoreTimestamps())
}

func TestAssertSnapshot_NilResult(t *testing.T) {
	dir := t.TempDir()

	// nil result should be handled gracefully
	AssertSnapshot(t, "nil-result", nil, WithSnapshotDir(dir))
	AssertSnapshot(t, "nil-result", nil, WithSnapshotDir(dir))
}

func TestAssertSnapshot_ErrorResult(t *testing.T) {
	dir := t.TempDir()

	result := registry.MakeErrorResult("something went wrong")

	AssertSnapshot(t, "error-result", result, WithSnapshotDir(dir))
	AssertSnapshot(t, "error-result", result, WithSnapshotDir(dir))
}

func TestAssertSnapshot_CorruptedGoldenFile(t *testing.T) {
	dir := t.TempDir()

	// Write an invalid JSON file as the golden file.
	path := filepath.Join(dir, "corrupted.golden.json")
	if err := os.WriteFile(path, []byte("{{ not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	result := registry.MakeTextResult("any content")
	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	AssertSnapshot(mockT, "corrupted", result, WithSnapshotDir(dir))
	if !failed {
		t.Error("AssertSnapshot should have failed on corrupted golden file")
	}
}

func TestNormaliseResult_Nil(t *testing.T) {
	cfg := &snapshotConfig{}
	got := normaliseResult(nil, cfg)
	if got != nil {
		t.Errorf("normaliseResult(nil) = %v, want nil", got)
	}
}

func TestNormaliseResult_TimestampStripping(t *testing.T) {
	// Build a result with structured content that contains timestamp-like fields.
	// We confirm the top-level map strips known timestamp keys.
	result := registry.MakeTextResult("ts-check")
	cfg := &snapshotConfig{ignoreTimestamps: true}
	got := normaliseResult(result, cfg)
	if got == nil {
		t.Fatal("normaliseResult returned nil for valid result")
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("normaliseResult returned %T, want map", got)
	}
	for _, field := range timestampFields {
		if _, exists := m[field]; exists {
			t.Errorf("field %q should have been stripped", field)
		}
	}
}

func TestNormaliseResult_StructuredContentTimestampStripping(t *testing.T) {
	// Structured content is a map that may contain timestamp fields.
	type Payload struct {
		Timestamp string `json:"timestamp"`
		Data      string `json:"data"`
	}
	payload := Payload{Timestamp: "2026-01-01T00:00:00Z", Data: "value"}
	result := registry.MakeStructuredResult(registry.MakeTextContent("ok"), payload)

	cfg := &snapshotConfig{ignoreTimestamps: true}
	got := normaliseResult(result, cfg)
	if got == nil {
		t.Fatal("normaliseResult returned nil")
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map", got)
	}
	// Verify the top-level structuredContent map had "timestamp" stripped.
	sc, hasSC := m["structuredContent"]
	if !hasSC {
		// No structured content in map means it was nil — test passes vacuously.
		return
	}
	scMap, ok := sc.(map[string]any)
	if !ok {
		// structuredContent is not a map (e.g., string/number) — non-map branch covered.
		return
	}
	if _, exists := scMap["timestamp"]; exists {
		t.Error("timestamp field should have been stripped from structuredContent")
	}
}

func TestAssertSnapshot_WithIgnoreTimestamps_StructuredContent(t *testing.T) {
	dir := t.TempDir()

	type Payload struct {
		CreatedAt string `json:"created_at"`
		Value     string `json:"value"`
	}
	p1 := Payload{CreatedAt: "2026-01-01", Value: "same"}
	p2 := Payload{CreatedAt: "2026-06-01", Value: "same"}

	r1 := registry.MakeStructuredResult(registry.MakeTextContent("ok"), p1)
	r2 := registry.MakeStructuredResult(registry.MakeTextContent("ok"), p2)

	// Create snapshot with p1
	AssertSnapshot(t, "sc-ts-test", r1, WithSnapshotDir(dir), WithIgnoreTimestamps())

	// p2 differs only in created_at which should be stripped → should match
	AssertSnapshot(t, "sc-ts-test", r2, WithSnapshotDir(dir), WithIgnoreTimestamps())
}
