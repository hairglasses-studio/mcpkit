package rdcycle

import (
	"context"
	"testing"
	"time"
)

func TestPerpetualToolsRegistered(t *testing.T) {
	mod := NewModule(CycleConfig{})
	tools := mod.Tools()

	expected := map[string]bool{
		"rdcycle_perpetual_start":  false,
		"rdcycle_perpetual_stop":   false,
		"rdcycle_perpetual_status": false,
	}

	for _, tool := range tools {
		if _, ok := expected[tool.Tool.Name]; ok {
			expected[tool.Tool.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("tool %q not found in Tools()", name)
		}
	}
}

func TestToolCount(t *testing.T) {
	mod := NewModule(CycleConfig{})
	tools := mod.Tools()
	if len(tools) != 12 {
		t.Errorf("expected 12 tools, got %d", len(tools))
	}
}

func TestPerpetualStartNoStarter(t *testing.T) {
	mod := NewModule(CycleConfig{})
	_, err := mod.handlePerpetualStart(context.Background(), perpetualStartInput{})
	if err == nil {
		t.Error("expected error when no RalphStarter configured")
	}
}

func TestPerpetualStartAndStop(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	mod.SetRalphStarter(func(ctx context.Context, specPath string) error {
		// Simulate a brief ralph run.
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	out, err := mod.handlePerpetualStart(context.Background(), perpetualStartInput{
		MaxCycles: 2,
	})
	if err != nil {
		t.Fatalf("start error: %v", err)
	}
	if !out.Started {
		t.Error("expected started=true")
	}

	// Wait for it to finish (max 2 cycles with brief sleep).
	time.Sleep(3 * time.Second)

	statusOut, err := mod.handlePerpetualStatus(context.Background(), perpetualStatusInput{})
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if statusOut.CycleNum < 1 {
		t.Error("expected at least 1 cycle")
	}
}

func TestPerpetualStartAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	// Simulate a long-running ralph.
	mod.SetRalphStarter(func(ctx context.Context, specPath string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
			return nil
		}
	})

	out1, _ := mod.handlePerpetualStart(context.Background(), perpetualStartInput{MaxCycles: 100})
	if !out1.Started {
		t.Fatal("first start should succeed")
	}

	// Wait for orchestrator goroutine to mark itself running.
	time.Sleep(100 * time.Millisecond)

	// Second start should be rejected.
	out2, err := mod.handlePerpetualStart(context.Background(), perpetualStartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out2.Started {
		t.Error("second start should return started=false")
	}

	// Clean up.
	mod.handlePerpetualStop(context.Background(), perpetualStopInput{})
}

func TestPerpetualStopNotRunning(t *testing.T) {
	mod := NewModule(CycleConfig{})
	out, err := mod.handlePerpetualStop(context.Background(), perpetualStopInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped {
		t.Error("expected stopped=false when nothing running")
	}
}

func TestPerpetualStatusNotStarted(t *testing.T) {
	mod := NewModule(CycleConfig{})
	out, err := mod.handlePerpetualStatus(context.Background(), perpetualStatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Running {
		t.Error("expected running=false when not started")
	}
}
