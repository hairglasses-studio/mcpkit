//go:build !official_sdk

package ralph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkpointEntry is a JSON-serializable form of ConversationTurn.
type checkpointEntry struct {
	UserPrompt    string   `json:"user_prompt"`
	AssistantText string   `json:"assistant_text"`
	ToolResults   []string `json:"tool_results,omitempty"`
}

// SaveCheckpoint atomically writes conversation history to disk (tmp+rename).
func SaveCheckpoint(path string, turns []ConversationTurn) error {
	entries := make([]checkpointEntry, len(turns))
	for i, t := range turns {
		entries[i] = checkpointEntry{
			UserPrompt:    t.UserPrompt,
			AssistantText: t.AssistantText,
			ToolResults:   t.ToolResults,
		}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("ralph: marshal checkpoint: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ralph-checkpoint-*.tmp")
	if err != nil {
		return fmt.Errorf("ralph: create temp checkpoint: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("ralph: write temp checkpoint: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ralph: close temp checkpoint: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ralph: rename checkpoint file: %w", err)
	}
	return nil
}

// LoadCheckpoint reads conversation history from disk.
// Returns nil (not error) if the file does not exist.
func LoadCheckpoint(path string) ([]ConversationTurn, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ralph: load checkpoint: %w", err)
	}
	var entries []checkpointEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("ralph: parse checkpoint: %w", err)
	}
	turns := make([]ConversationTurn, len(entries))
	for i, e := range entries {
		turns[i] = ConversationTurn{
			UserPrompt:    e.UserPrompt,
			AssistantText: e.AssistantText,
			ToolResults:   e.ToolResults,
		}
	}
	return turns, nil
}

// DefaultCheckpointFile returns the default checkpoint path for a spec file.
// E.g., "specs/task.json" → "specs/task.checkpoint.json"
// E.g., "task.yaml" → "task.checkpoint.yaml"
func DefaultCheckpointFile(specFile string) string {
	ext := filepath.Ext(specFile)
	return strings.TrimSuffix(specFile, ext) + ".checkpoint" + ext
}

// pruneHistory limits conversation history to maxTurns, keeping the most recent.
func pruneHistory(turns []ConversationTurn, maxTurns int) []ConversationTurn {
	if maxTurns <= 0 || len(turns) <= maxTurns {
		return turns
	}
	return turns[len(turns)-maxTurns:]
}
