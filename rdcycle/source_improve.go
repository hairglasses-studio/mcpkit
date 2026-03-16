package rdcycle

import (
	"context"
	"fmt"
	"path/filepath"
)

// ImprovementSource generates candidate tasks from improvement notes.
type ImprovementSource struct {
	NotesPath string
}

// NewImprovementSource creates an ImprovementSource.
func NewImprovementSource(notesPath string) *ImprovementSource {
	if notesPath == "" {
		notesPath = filepath.Join("rdcycle", "notes", "improvement_log.json")
	}
	return &ImprovementSource{NotesPath: notesPath}
}

// Fetch returns candidate tasks based on improvement history.
func (is *ImprovementSource) Fetch(_ context.Context) ([]CandidateTask, error) {
	notes, _ := LoadNotes(is.NotesPath)
	if len(notes) == 0 {
		return nil, nil
	}

	var candidates []CandidateTask

	if len(notes)%10 == 0 {
		candidates = append(candidates, CandidateTask{
			ID:          "self_improve",
			Description: "Run rdcycle_improve to analyze accumulated notes and apply recommendations.",
			Source:      "improvement",
			Priority:    50,
			Complexity:  "simple",
		})
	}

	if len(notes) >= 3 {
		recent := notes[len(notes)-3:]
		failCounts := make(map[string]int)
		for _, n := range recent {
			for _, f := range n.WhatFailed {
				failCounts[f]++
			}
		}
		priority := 20
		for failure, count := range failCounts {
			if count >= 2 {
				candidates = append(candidates, CandidateTask{
					ID:          fmt.Sprintf("fix_%d", priority),
					Description: fmt.Sprintf("Fix recurring issue: %s (appeared in %d/3 recent cycles).", failure, count),
					Source:      "improvement",
					Priority:    priority,
					Complexity:  "moderate",
				})
				priority++
			}
		}
	}

	return candidates, nil
}
