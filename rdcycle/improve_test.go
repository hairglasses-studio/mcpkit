package rdcycle

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestHandleImprove_EmptyNotes(t *testing.T) {
	dir := t.TempDir()
	m := NewModule(CycleConfig{})

	out, err := m.handleImprove(context.Background(), ImproveInput{
		NotesPath: filepath.Join(dir, "empty.json"),
	})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}
	if out.CyclesAnalyzed != 0 {
		t.Errorf("cycles_analyzed = %d, want 0", out.CyclesAnalyzed)
	}
	if len(out.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}
}

func TestHandleImprove_PatternAnalysis(t *testing.T) {
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	notes := []ImprovementNote{
		{CycleID: "c1", CycleNumber: 1, WhatWorked: []string{"fast"}, WhatFailed: []string{"flaky tests", "timeout"}, WastedIters: 5, TotalCost: 2.0},
		{CycleID: "c2", CycleNumber: 2, WhatWorked: []string{"fast"}, WhatFailed: []string{"flaky tests"}, WastedIters: 3, TotalCost: 1.5},
		{CycleID: "c3", CycleNumber: 3, WhatWorked: []string{"clean"}, WhatFailed: []string{"flaky tests", "timeout"}, WastedIters: 4, TotalCost: 1.0},
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	if out.CyclesAnalyzed != 3 {
		t.Errorf("cycles_analyzed = %d, want 3", out.CyclesAnalyzed)
	}
	if len(out.CommonFailures) == 0 {
		t.Error("expected common failures to be detected")
	}
	if out.AvgWastedIters != 4.0 {
		t.Errorf("avg_wasted_iters = %f, want 4.0", out.AvgWastedIters)
	}
}

func TestHandleImprove_CostTrend(t *testing.T) {
	tests := []struct {
		name  string
		costs []float64
		want  string
	}{
		{"increasing", []float64{1.0, 2.0, 3.0}, "increasing"},
		{"decreasing", []float64{3.0, 2.0, 1.0}, "decreasing"},
		{"stable", []float64{2.0, 2.0, 2.0}, "stable"},
		{"single", []float64{1.0}, "stable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notes []ImprovementNote
			for i, c := range tt.costs {
				notes = append(notes, ImprovementNote{
					CycleID:     fmt.Sprintf("c%d", i),
					CycleNumber: i + 1,
					TotalCost:   c,
				})
			}
			got := analyzeCostTrend(notes)
			if got != tt.want {
				t.Errorf("analyzeCostTrend = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleImprove_BudgetSuggestion(t *testing.T) {
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.json")

	// 5 cycles with increasing costs should trigger a budget suggestion.
	var notes []ImprovementNote
	for i := 0; i < 5; i++ {
		notes = append(notes, ImprovementNote{
			CycleID:     fmt.Sprintf("c%d", i),
			CycleNumber: i + 1,
			TotalCost:   float64(i+1) * 2.0,
			WhatWorked:  []string{"ok"},
			WhatFailed:  []string{"cost"},
		})
	}
	if err := SaveNotes(notesPath, notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}

	m := NewModule(CycleConfig{})
	out, err := m.handleImprove(context.Background(), ImproveInput{NotesPath: notesPath})
	if err != nil {
		t.Fatalf("handleImprove: %v", err)
	}

	if out.BudgetSuggestion == nil {
		t.Error("expected budget suggestion for increasing cost trend with 5+ cycles")
	}
	if out.CostTrend != "increasing" {
		t.Errorf("cost_trend = %q, want increasing", out.CostTrend)
	}
}
