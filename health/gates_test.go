package health

import (
	"math"
	"sync"
	"testing"
)

func TestGateRegistry_Register(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:        "test_gate",
		Description: "A test gate",
		Threshold:   0.95,
	})

	names := r.Gates()
	if len(names) != 1 {
		t.Fatalf("gate count = %d, want 1", len(names))
	}
	if names[0] != "test_gate" {
		t.Errorf("gate name = %q, want test_gate", names[0])
	}
}

func TestGateRegistry_Register_Replace(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:        "test_gate",
		Description: "Original",
		Threshold:   0.50,
	})
	r.Register(AutonomyGate{
		Name:        "test_gate",
		Description: "Replaced",
		Threshold:   0.90,
	})

	names := r.Gates()
	if len(names) != 1 {
		t.Fatalf("gate count = %d, want 1 (replace should not duplicate)", len(names))
	}

	results := r.CheckAll()
	if results["test_gate"].Threshold != 0.90 {
		t.Errorf("threshold = %f, want 0.90", results["test_gate"].Threshold)
	}
	if results["test_gate"].Description != "Replaced" {
		t.Errorf("description = %q, want Replaced", results["test_gate"].Description)
	}
}

func TestGateRegistry_Register_PreservesOrder(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{Name: "alpha"})
	r.Register(AutonomyGate{Name: "beta"})
	r.Register(AutonomyGate{Name: "gamma"})

	names := r.Gates()
	want := []string{"alpha", "beta", "gamma"}
	if len(names) != len(want) {
		t.Fatalf("gate count = %d, want %d", len(names), len(want))
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("gates[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestGateRegistry_Check_Passing(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:      "good_gate",
		Threshold: 0.95,
		Measure:   func() float64 { return 0.98 },
		Passing:   func() bool { return true },
	})

	value, passing := r.Check("good_gate")
	if value != 0.98 {
		t.Errorf("value = %f, want 0.98", value)
	}
	if !passing {
		t.Error("passing = false, want true")
	}
}

func TestGateRegistry_Check_Failing(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:      "bad_gate",
		Threshold: 0.95,
		Measure:   func() float64 { return 0.22 },
		Passing:   func() bool { return false },
	})

	value, passing := r.Check("bad_gate")
	if value != 0.22 {
		t.Errorf("value = %f, want 0.22", value)
	}
	if passing {
		t.Error("passing = true, want false")
	}
}

func TestGateRegistry_Check_NotFound(t *testing.T) {
	r := NewGateRegistry()
	value, passing := r.Check("nonexistent")
	if value != -1 {
		t.Errorf("value = %f, want -1", value)
	}
	if passing {
		t.Error("passing = true, want false for nonexistent gate")
	}
}

func TestGateRegistry_Check_Unmeasured(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:      "unmeasured_gate",
		Threshold: 0.95,
		// nil Measure and Passing
	})

	value, passing := r.Check("unmeasured_gate")
	if value != -1 {
		t.Errorf("value = %f, want -1 for unmeasured gate", value)
	}
	if passing {
		t.Error("passing = true, want false for unmeasured gate")
	}
}

func TestGateRegistry_CheckAll(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:      "passing_gate",
		Threshold: 0.95,
		Measure:   func() float64 { return 0.99 },
		Passing:   func() bool { return true },
	})
	r.Register(AutonomyGate{
		Name:      "failing_gate",
		Threshold: 0.95,
		Measure:   func() float64 { return 0.50 },
		Passing:   func() bool { return false },
	})
	r.Register(AutonomyGate{
		Name:      "unmeasured_gate",
		Threshold: 0.80,
	})

	results := r.CheckAll()

	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}

	// Check passing gate
	pg := results["passing_gate"]
	if pg.Status != GatePass {
		t.Errorf("passing_gate status = %q, want PASS", pg.Status)
	}
	if pg.Value != 0.99 {
		t.Errorf("passing_gate value = %f, want 0.99", pg.Value)
	}
	if pg.Threshold != 0.95 {
		t.Errorf("passing_gate threshold = %f, want 0.95", pg.Threshold)
	}

	// Check failing gate
	fg := results["failing_gate"]
	if fg.Status != GateFail {
		t.Errorf("failing_gate status = %q, want FAIL", fg.Status)
	}
	if fg.Value != 0.50 {
		t.Errorf("failing_gate value = %f, want 0.50", fg.Value)
	}

	// Check unmeasured gate
	ug := results["unmeasured_gate"]
	if ug.Status != GateUnmeasured {
		t.Errorf("unmeasured_gate status = %q, want UNMEASURED", ug.Status)
	}
	if ug.Value != -1 {
		t.Errorf("unmeasured_gate value = %f, want -1", ug.Value)
	}
}

func TestGateRegistry_Score_AllPassing(t *testing.T) {
	r := NewGateRegistry()
	for _, name := range []string{"a", "b", "c"} {
		r.Register(AutonomyGate{
			Name:    name,
			Passing: func() bool { return true },
		})
	}

	score := r.Score()
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func TestGateRegistry_Score_NonePassing(t *testing.T) {
	r := NewGateRegistry()
	for _, name := range []string{"a", "b", "c"} {
		r.Register(AutonomyGate{
			Name:    name,
			Passing: func() bool { return false },
		})
	}

	score := r.Score()
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0", score)
	}
}

func TestGateRegistry_Score_Mixed(t *testing.T) {
	r := NewGateRegistry()
	r.Register(AutonomyGate{
		Name:    "pass1",
		Passing: func() bool { return true },
	})
	r.Register(AutonomyGate{
		Name:    "pass2",
		Passing: func() bool { return true },
	})
	r.Register(AutonomyGate{
		Name:    "fail1",
		Passing: func() bool { return false },
	})
	r.Register(AutonomyGate{
		Name: "unmeasured",
		// nil Passing counts as failing
	})

	score := r.Score()
	// 2 passing out of 4 total = 0.5
	if score != 0.5 {
		t.Errorf("score = %f, want 0.5", score)
	}
}

func TestGateRegistry_Score_Empty(t *testing.T) {
	r := NewGateRegistry()
	score := r.Score()
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0 for empty registry", score)
	}
}

func TestGateRegistry_Score_Aggregate(t *testing.T) {
	r := NewGateRegistry()
	// 3 passing out of 7 total = 3/7 ≈ 0.4286
	for i, name := range []string{"g1", "g2", "g3", "g4", "g5", "g6", "g7"} {
		pass := i < 3
		r.Register(AutonomyGate{
			Name:    name,
			Passing: func() bool { return pass },
		})
	}

	score := r.Score()
	expected := 3.0 / 7.0
	if math.Abs(score-expected) > 1e-9 {
		t.Errorf("score = %f, want %f", score, expected)
	}
}

func TestDefaultGates_AllDefined(t *testing.T) {
	gates := DefaultGates()

	if len(gates) != 7 {
		t.Fatalf("DefaultGates count = %d, want 7", len(gates))
	}

	expectedNames := []string{
		"json_retry_rate",
		"session_completion_rate",
		"cost_per_task",
		"crash_recovery_success",
		"budget_enforcement_compliance",
		"context_compaction_success",
		"fleet_health_score",
	}

	for i, gate := range gates {
		if gate.Name != expectedNames[i] {
			t.Errorf("gate[%d].Name = %q, want %q", i, gate.Name, expectedNames[i])
		}
		if gate.Description == "" {
			t.Errorf("gate[%d] (%s) has empty description", i, gate.Name)
		}
		if gate.Threshold <= 0 || gate.Threshold > 1.0 {
			t.Errorf("gate[%d] (%s) threshold = %f, want (0.0, 1.0]", i, gate.Name, gate.Threshold)
		}
		// Default gates should have nil Measure/Passing (unmeasured)
		if gate.Measure != nil {
			t.Errorf("gate[%d] (%s) Measure should be nil (unmeasured by default)", i, gate.Name)
		}
		if gate.Passing != nil {
			t.Errorf("gate[%d] (%s) Passing should be nil (unmeasured by default)", i, gate.Name)
		}
	}
}

func TestDefaultGates_RegisterAll(t *testing.T) {
	r := NewGateRegistry()
	for _, gate := range DefaultGates() {
		r.Register(gate)
	}

	names := r.Gates()
	if len(names) != 7 {
		t.Fatalf("registered gate count = %d, want 7", len(names))
	}

	// All should be unmeasured
	results := r.CheckAll()
	for name, result := range results {
		if result.Status != GateUnmeasured {
			t.Errorf("gate %q status = %q, want UNMEASURED", name, result.Status)
		}
	}

	// Score should be 0 since nothing is measured
	score := r.Score()
	if score != 0.0 {
		t.Errorf("score = %f, want 0.0 for all unmeasured gates", score)
	}
}

func TestDefaultGates_UniqueNames(t *testing.T) {
	gates := DefaultGates()
	seen := make(map[string]bool)
	for _, gate := range gates {
		if seen[gate.Name] {
			t.Errorf("duplicate gate name: %q", gate.Name)
		}
		seen[gate.Name] = true
	}
}

func TestDefaultGates_Thresholds(t *testing.T) {
	gates := DefaultGates()
	expected := map[string]float64{
		"json_retry_rate":               0.05,
		"session_completion_rate":       0.95,
		"cost_per_task":                 0.50,
		"crash_recovery_success":        0.99,
		"budget_enforcement_compliance": 1.0,
		"context_compaction_success":    0.90,
		"fleet_health_score":            0.80,
	}

	for _, gate := range gates {
		want, ok := expected[gate.Name]
		if !ok {
			t.Errorf("unexpected gate name: %q", gate.Name)
			continue
		}
		if gate.Threshold != want {
			t.Errorf("gate %q threshold = %f, want %f", gate.Name, gate.Threshold, want)
		}
	}
}

func TestGateRegistry_Concurrent(t *testing.T) {
	r := NewGateRegistry()

	// Pre-register a gate for concurrent Check calls
	r.Register(AutonomyGate{
		Name:      "concurrent_gate",
		Threshold: 0.90,
		Measure:   func() float64 { return 0.95 },
		Passing:   func() bool { return true },
	})

	var wg sync.WaitGroup
	// Concurrent registrations
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Register(AutonomyGate{
				Name:      "concurrent_gate",
				Threshold: float64(n) / 100.0,
				Measure:   func() float64 { return float64(n) / 100.0 },
				Passing:   func() bool { return n > 50 },
			})
		}(i)
	}

	// Concurrent reads
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Check("concurrent_gate")
			r.CheckAll()
			r.Score()
			r.Gates()
		}()
	}

	wg.Wait()
}
