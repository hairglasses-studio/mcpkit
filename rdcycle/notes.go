package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ImprovementNote records observations from a single R&D cycle for
// later analysis by the self-improvement tool.
type ImprovementNote struct {
	CycleID     string   `json:"cycle_id"`
	CycleNumber int      `json:"cycle_number"`
	WhatWorked  []string `json:"what_worked"`
	WhatFailed  []string `json:"what_failed"`
	WastedIters int      `json:"wasted_iterations"`
	TotalCost   float64  `json:"total_cost_dollars"`
	Suggestions []string `json:"suggestions"`
	Timestamp   string   `json:"timestamp"`
}

// NotesInput is the input for the rdcycle_notes tool.
type NotesInput struct {
	CycleID     string   `json:"cycle_id" jsonschema:"required,description=Unique identifier for this cycle"`
	CycleNumber int      `json:"cycle_number" jsonschema:"required,description=Sequential cycle number"`
	WhatWorked  []string `json:"what_worked" jsonschema:"required,description=Things that went well this cycle"`
	WhatFailed  []string `json:"what_failed" jsonschema:"required,description=Things that went wrong or were wasteful"`
	WastedIters int      `json:"wasted_iterations,omitempty" jsonschema:"description=Number of iterations that produced no useful output"`
	TotalCost   float64  `json:"total_cost_dollars,omitempty" jsonschema:"description=Total dollar cost of this cycle"`
	Suggestions []string `json:"suggestions,omitempty" jsonschema:"description=Suggestions for improving the next cycle"`
	NotesPath   string   `json:"notes_path,omitempty" jsonschema:"description=Path to improvement log file (default: rdcycle/notes/improvement_log.json)"`
}

// NotesOutput is the output of the rdcycle_notes tool.
type NotesOutput struct {
	Saved     bool   `json:"saved"`
	NoteCount int    `json:"note_count"`
	NotesPath string `json:"notes_path"`
}

func (m *Module) notesTool() registry.ToolDefinition {
	desc := "Record improvement notes for an R&D cycle: what worked, what failed, " +
		"wasted iterations, cost, and suggestions. Notes are persisted to disk and " +
		"to the artifact store for later analysis by rdcycle_improve."

	td := handler.TypedHandler[NotesInput, NotesOutput](
		"rdcycle_notes",
		desc,
		m.handleNotes,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.IsWrite = true
	return td
}

func (m *Module) handleNotes(_ context.Context, input NotesInput) (NotesOutput, error) {
	if input.CycleID == "" {
		return NotesOutput{}, fmt.Errorf("cycle_id is required")
	}

	note := ImprovementNote{
		CycleID:     input.CycleID,
		CycleNumber: input.CycleNumber,
		WhatWorked:  input.WhatWorked,
		WhatFailed:  input.WhatFailed,
		WastedIters: input.WastedIters,
		TotalCost:   input.TotalCost,
		Suggestions: input.Suggestions,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	notesPath := input.NotesPath
	if notesPath == "" {
		notesPath = filepath.Join("rdcycle", "notes", "improvement_log.json")
	}

	// Load existing notes.
	notes, err := LoadNotes(notesPath)
	if err != nil {
		notes = []ImprovementNote{}
	}
	notes = append(notes, note)

	if err := SaveNotes(notesPath, notes); err != nil {
		return NotesOutput{}, fmt.Errorf("save notes: %w", err)
	}

	// Also save to artifact store.
	content := map[string]any{
		"cycle_id":     note.CycleID,
		"cycle_number": note.CycleNumber,
		"what_worked":  note.WhatWorked,
		"what_failed":  note.WhatFailed,
		"wasted_iters": note.WastedIters,
		"total_cost":   note.TotalCost,
		"suggestions":  note.Suggestions,
	}
	m.store.Save(Artifact{
		ID:        artifactID("improvement"),
		Type:      "improvement",
		Content:   content,
		CreatedAt: note.Timestamp,
	})

	return NotesOutput{
		Saved:     true,
		NoteCount: len(notes),
		NotesPath: notesPath,
	}, nil
}

// LoadNotes reads improvement notes from a JSON file.
// Returns an empty slice if the file does not exist.
func LoadNotes(path string) ([]ImprovementNote, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("rdcycle: load notes: %w", err)
	}
	var notes []ImprovementNote
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, fmt.Errorf("rdcycle: parse notes: %w", err)
	}
	return notes, nil
}

// SaveNotes writes improvement notes to a JSON file, creating parent dirs if needed.
func SaveNotes(path string, notes []ImprovementNote) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("rdcycle: create notes dir: %w", err)
	}
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return fmt.Errorf("rdcycle: marshal notes: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("rdcycle: write notes: %w", err)
	}
	return nil
}
