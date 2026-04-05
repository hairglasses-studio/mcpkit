package rdcycle

import (
	"fmt"
	"strings"
)

// TaskMutation describes a modification the synthesizer should apply to the task DAG.
type TaskMutation struct {
	// Action is "skip", "add_verify", or "meta_improve".
	Action string
	// TaskID is the task this mutation applies to (empty for "meta_improve").
	TaskID string
	// Reason explains why this mutation was suggested.
	Reason string
}

// LearningEngine extracts structured signals from improvement notes for the synthesizer.
type LearningEngine struct {
	notesPath string
}

// NewLearningEngine creates a LearningEngine that reads notes from the given path.
func NewLearningEngine(notesPath string) *LearningEngine {
	return &LearningEngine{notesPath: notesPath}
}

// AvoidPatterns returns failure patterns appearing in >25% of the last windowSize cycles.
func (le *LearningEngine) AvoidPatterns(windowSize int) []string {
	notes, err := LoadNotes(le.notesPath)
	if err != nil || len(notes) == 0 {
		return nil
	}

	if windowSize <= 0 {
		windowSize = 10
	}
	start := max(len(notes)-windowSize, 0)
	window := notes[start:]

	failCounts := make(map[string]int)
	for _, n := range window {
		for _, f := range n.WhatFailed {
			failCounts[f]++
		}
	}

	threshold := max(1, len(window)/4)
	var patterns []string
	for failure, count := range failCounts {
		if count >= threshold {
			patterns = append(patterns, failure)
		}
	}
	return patterns
}

// CostTrend returns the cost trend from recent cycles: "increasing", "decreasing", or "stable".
func (le *LearningEngine) CostTrend() string {
	notes, err := LoadNotes(le.notesPath)
	if err != nil || len(notes) == 0 {
		return "stable"
	}
	return analyzeCostTrend(notes)
}

// TaskMutations returns suggested modifications for the next cycle's task DAG.
func (le *LearningEngine) TaskMutations() []TaskMutation {
	notes, err := LoadNotes(le.notesPath)
	if err != nil || len(notes) == 0 {
		return nil
	}

	var mutations []TaskMutation

	taskFailCounts := make(map[string]int)
	verifyFailCount := 0
	for _, n := range notes {
		for _, f := range n.WhatFailed {
			taskFailCounts[f]++
			lower := strings.ToLower(f)
			if strings.Contains(lower, "verify") || strings.Contains(lower, "test") ||
				strings.Contains(lower, "build") || strings.Contains(lower, "compile") {
				verifyFailCount++
			}
		}
	}

	// Skip tasks matching recurring failures (>50% of cycles).
	threshold := max(1, len(notes)/2)
	for failure, count := range taskFailCounts {
		if count >= threshold {
			mutations = append(mutations, TaskMutation{
				Action: "skip",
				TaskID: failure,
				Reason: fmt.Sprintf("%d/%d cycles failed on this", count, len(notes)),
			})
		}
	}

	// Add extra verify step after implement if verification frequently fails.
	if verifyFailCount > len(notes)/3 {
		mutations = append(mutations, TaskMutation{
			Action: "add_verify",
			TaskID: "implement",
			Reason: fmt.Sprintf("verification fails in %d/%d cycles", verifyFailCount, len(notes)),
		})
	}

	// Meta-improve every 10 cycles.
	if len(notes)%10 == 0 {
		mutations = append(mutations, TaskMutation{
			Action: "meta_improve",
			Reason: fmt.Sprintf("accumulated %d cycles of notes", len(notes)),
		})
	}

	return mutations
}
