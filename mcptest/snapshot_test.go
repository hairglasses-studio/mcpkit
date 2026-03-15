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
