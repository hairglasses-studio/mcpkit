package rdcycle

import (
	"fmt"
	"path/filepath"
	"testing"
)

func testNotesPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "notes.json")
}

func writeTestNotes(t *testing.T, path string, notes []ImprovementNote) {
	t.Helper()
	if err := SaveNotes(path, notes); err != nil {
		t.Fatalf("write test notes: %v", err)
	}
}

func TestLearningEngine_AvoidPatterns_Empty(t *testing.T) {
	le := NewLearningEngine(filepath.Join(t.TempDir(), "nonexistent.json"))
	patterns := le.AvoidPatterns(10)
	if len(patterns) != 0 {
		t.Fatalf("expected no patterns, got %v", patterns)
	}
}

func TestLearningEngine_AvoidPatterns_Threshold(t *testing.T) {
	path := testNotesPath(t)
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"build timeout", "flaky test"}},
		{CycleID: "c2", WhatFailed: []string{"build timeout"}},
		{CycleID: "c3", WhatFailed: []string{"build timeout", "OOM"}},
		{CycleID: "c4", WhatFailed: []string{"flaky test"}},
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	patterns := le.AvoidPatterns(10)
	found := make(map[string]bool)
	for _, p := range patterns {
		found[p] = true
	}
	if !found["build timeout"] {
		t.Fatal("expected 'build timeout' in avoid patterns")
	}
	if !found["flaky test"] {
		t.Fatal("expected 'flaky test' in avoid patterns")
	}
}

func TestLearningEngine_AvoidPatterns_WindowSize(t *testing.T) {
	path := testNotesPath(t)
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"old issue"}},
		{CycleID: "c2", WhatFailed: []string{"old issue"}},
		{CycleID: "c3", WhatFailed: []string{"new issue"}},
		{CycleID: "c4", WhatFailed: []string{"new issue"}},
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	patterns := le.AvoidPatterns(2)
	found := make(map[string]bool)
	for _, p := range patterns {
		found[p] = true
	}
	if found["old issue"] {
		t.Fatal("old issue should not appear in window of 2")
	}
	if !found["new issue"] {
		t.Fatal("expected 'new issue' in avoid patterns")
	}
}

func TestLearningEngine_CostTrend_Empty(t *testing.T) {
	le := NewLearningEngine(filepath.Join(t.TempDir(), "nonexistent.json"))
	if trend := le.CostTrend(); trend != "stable" {
		t.Fatalf("expected stable, got %s", trend)
	}
}

func TestLearningEngine_CostTrend_Increasing(t *testing.T) {
	path := testNotesPath(t)
	notes := []ImprovementNote{
		{CycleID: "c1", TotalCost: 1.0},
		{CycleID: "c2", TotalCost: 2.0},
		{CycleID: "c3", TotalCost: 3.0},
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	if trend := le.CostTrend(); trend != "increasing" {
		t.Fatalf("expected increasing, got %s", trend)
	}
}

func TestLearningEngine_TaskMutations_Empty(t *testing.T) {
	le := NewLearningEngine(filepath.Join(t.TempDir(), "nonexistent.json"))
	mutations := le.TaskMutations()
	if len(mutations) != 0 {
		t.Fatalf("expected no mutations, got %v", mutations)
	}
}

func TestLearningEngine_TaskMutations_SkipRecurring(t *testing.T) {
	path := testNotesPath(t)
	var notes []ImprovementNote
	for i := 0; i < 4; i++ {
		n := ImprovementNote{CycleID: fmt.Sprintf("c%d", i)}
		if i < 3 {
			n.WhatFailed = []string{"scan flake"}
		}
		notes = append(notes, n)
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	mutations := le.TaskMutations()
	var foundSkip bool
	for _, m := range mutations {
		if m.Action == "skip" && m.TaskID == "scan flake" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatal("expected skip mutation for recurring failure")
	}
}

func TestLearningEngine_TaskMutations_AddVerify(t *testing.T) {
	path := testNotesPath(t)
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"verify failed"}},
		{CycleID: "c2", WhatFailed: []string{"build broke"}},
		{CycleID: "c3", WhatFailed: []string{"test flake"}},
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	mutations := le.TaskMutations()
	var foundAddVerify bool
	for _, m := range mutations {
		if m.Action == "add_verify" {
			foundAddVerify = true
		}
	}
	if !foundAddVerify {
		t.Fatal("expected add_verify mutation when verification frequently fails")
	}
}

func TestLearningEngine_TaskMutations_MetaImprove(t *testing.T) {
	path := testNotesPath(t)
	var notes []ImprovementNote
	for i := 0; i < 10; i++ {
		notes = append(notes, ImprovementNote{CycleID: fmt.Sprintf("c%d", i)})
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	mutations := le.TaskMutations()
	var foundMeta bool
	for _, m := range mutations {
		if m.Action == "meta_improve" {
			foundMeta = true
		}
	}
	if !foundMeta {
		t.Fatal("expected meta_improve mutation at 10 cycles")
	}
}

func TestLearningEngine_AvoidPatterns_DefaultWindow(t *testing.T) {
	path := testNotesPath(t)
	notes := []ImprovementNote{
		{CycleID: "c1", WhatFailed: []string{"issue"}},
	}
	writeTestNotes(t, path, notes)

	le := NewLearningEngine(path)
	patterns := le.AvoidPatterns(0)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
}
