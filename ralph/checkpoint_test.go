//go:build !official_sdk

package ralph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadCheckpoint_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.checkpoint.json")

	turns := []ConversationTurn{
		{UserPrompt: "prompt1", AssistantText: "response1", ToolResults: []string{"result1"}},
		{UserPrompt: "prompt2", AssistantText: "response2", ToolResults: nil},
	}

	if err := SaveCheckpoint(path, turns); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	loaded, err := LoadCheckpoint(path)
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}

	if len(loaded) != len(turns) {
		t.Fatalf("got %d turns, want %d", len(loaded), len(turns))
	}
	for i, got := range loaded {
		want := turns[i]
		if got.UserPrompt != want.UserPrompt {
			t.Errorf("turn[%d].UserPrompt = %q, want %q", i, got.UserPrompt, want.UserPrompt)
		}
		if got.AssistantText != want.AssistantText {
			t.Errorf("turn[%d].AssistantText = %q, want %q", i, got.AssistantText, want.AssistantText)
		}
		if len(got.ToolResults) != len(want.ToolResults) {
			t.Errorf("turn[%d].ToolResults length = %d, want %d", i, len(got.ToolResults), len(want.ToolResults))
		}
	}
}

func TestLoadCheckpoint_MissingFile(t *testing.T) {
	t.Parallel()
	turns, err := LoadCheckpoint("/nonexistent/path/checkpoint.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if turns != nil {
		t.Errorf("expected nil turns for missing file, got %d", len(turns))
	}
}

func TestLoadCheckpoint_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.checkpoint.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadCheckpoint(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveCheckpoint_EmptyTurns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.checkpoint.json")

	if err := SaveCheckpoint(path, nil); err != nil {
		t.Fatalf("SaveCheckpoint with nil: %v", err)
	}

	loaded, err := LoadCheckpoint(path)
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 turns, got %d", len(loaded))
	}
}

func TestSaveCheckpoint_AtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.checkpoint.json")

	// Write initial data.
	turns1 := []ConversationTurn{
		{UserPrompt: "first", AssistantText: "resp1"},
	}
	if err := SaveCheckpoint(path, turns1); err != nil {
		t.Fatal(err)
	}

	// Overwrite with new data.
	turns2 := []ConversationTurn{
		{UserPrompt: "first", AssistantText: "resp1"},
		{UserPrompt: "second", AssistantText: "resp2"},
	}
	if err := SaveCheckpoint(path, turns2); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 turns after overwrite, got %d", len(loaded))
	}
}

func TestDefaultCheckpointFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"specs/task.json", "specs/task.checkpoint.json"},
		{"task.yaml", "task.checkpoint.yaml"},
		{"my.spec.json", "my.spec.checkpoint.json"},
	}
	for _, tt := range tests {
		got := DefaultCheckpointFile(tt.input)
		if got != tt.want {
			t.Errorf("DefaultCheckpointFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPruneHistory(t *testing.T) {
	t.Parallel()
	turns := make([]ConversationTurn, 10)
	for i := range turns {
		turns[i] = ConversationTurn{UserPrompt: "p"}
	}

	// Prune to 5 — should keep last 5.
	pruned := pruneHistory(turns, 5)
	if len(pruned) != 5 {
		t.Errorf("pruneHistory(10, 5) = %d turns, want 5", len(pruned))
	}

	// No prune needed.
	noPrune := pruneHistory(turns, 20)
	if len(noPrune) != 10 {
		t.Errorf("pruneHistory(10, 20) = %d turns, want 10", len(noPrune))
	}

	// Zero max — no prune.
	zeroPrune := pruneHistory(turns, 0)
	if len(zeroPrune) != 10 {
		t.Errorf("pruneHistory(10, 0) = %d turns, want 10", len(zeroPrune))
	}
}
