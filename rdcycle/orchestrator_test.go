package rdcycle

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/roadmap"
)

func TestNewOrchestratorDefaults(t *testing.T) {
	mod := NewModule(CycleConfig{})
	o := NewOrchestrator(mod, OrchestratorConfig{})
	if o.cfg.ImproveCadence != 5 {
		t.Errorf("expected default improve cadence 5, got %d", o.cfg.ImproveCadence)
	}
}

func TestOrchestratorRunOneCycle(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	var ralphCalled atomic.Int32
	cfg := OrchestratorConfig{
		SpecDir: filepath.Join(dir, "specs"),
		RalphStarter: func(ctx context.Context, specPath string) error {
			ralphCalled.Add(1)
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	result, err := o.RunOneCycle(context.Background())
	if err != nil {
		t.Fatalf("RunOneCycle error: %v", err)
	}

	if result.CycleNum != 1 {
		t.Errorf("expected cycle 1, got %d", result.CycleNum)
	}
	if !result.Progress {
		t.Error("expected progress=true")
	}
	if result.SpecPath == "" {
		t.Error("expected spec path to be set")
	}
	if ralphCalled.Load() != 1 {
		t.Errorf("expected ralph called once, got %d", ralphCalled.Load())
	}
}

func TestOrchestratorRunMaxCycles(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	var cycleCount atomic.Int32
	cfg := OrchestratorConfig{
		SpecDir:   filepath.Join(dir, "specs"),
		MaxCycles: 3,
		RalphStarter: func(ctx context.Context, specPath string) error {
			cycleCount.Add(1)
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if got := cycleCount.Load(); got != 3 {
		t.Errorf("expected 3 cycles, got %d", got)
	}
	if o.CycleNum() != 3 {
		t.Errorf("expected cycle num 3, got %d", o.CycleNum())
	}
}

func TestOrchestratorBreakerStops(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	breaker := NewCircuitBreaker(1, time.Hour)
	breaker.RecordResult(false, false) // open the breaker

	cfg := OrchestratorConfig{
		SpecDir:  filepath.Join(dir, "specs"),
		Breaker:  breaker,
		MaxCycles: 10,
		RalphStarter: func(ctx context.Context, specPath string) error {
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	result, err := o.RunOneCycle(context.Background())
	if err == nil {
		t.Fatal("expected error when breaker is open")
	}
	if result.Progress {
		t.Error("expected no progress when breaker is open")
	}
}

func TestOrchestratorGovernorHalts(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	governor := NewCostVelocityGovernor(3, 100.0, 2) // halt after 2 unproductive
	governor.Record(CycleRecord{CycleNum: 1, Cost: 1.0, Productive: false})
	governor.Record(CycleRecord{CycleNum: 2, Cost: 1.0, Productive: false})

	cfg := OrchestratorConfig{
		SpecDir:   filepath.Join(dir, "specs"),
		Governor:  governor,
		MaxCycles: 10,
		RalphStarter: func(ctx context.Context, specPath string) error {
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	result, err := o.RunOneCycle(context.Background())
	if err == nil {
		t.Fatal("expected error when governor halts")
	}
	if result.Progress {
		t.Error("expected no progress when governor halts")
	}
}

func TestOrchestratorContextCancellation(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	ctx, cancel := context.WithCancel(context.Background())

	var cycleCount atomic.Int32
	cfg := OrchestratorConfig{
		SpecDir: filepath.Join(dir, "specs"),
		RalphStarter: func(ctx context.Context, specPath string) error {
			cycleCount.Add(1)
			if cycleCount.Load() >= 2 {
				cancel()
			}
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	err := o.Run(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestOrchestratorStop(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	var cycleCount atomic.Int32
	cfg := OrchestratorConfig{
		SpecDir: filepath.Join(dir, "specs"),
		RalphStarter: func(ctx context.Context, specPath string) error {
			cycleCount.Add(1)
			return nil
		},
		OnCycleEnd: func(cycleNum int, result CycleResult) {},
	}

	o := NewOrchestrator(mod, cfg)

	// Stop after first cycle via OnCycleEnd callback.
	cfg.OnCycleEnd = func(cycleNum int, result CycleResult) {
		if cycleNum >= 1 {
			o.Stop()
		}
	}
	o.cfg.OnCycleEnd = cfg.OnCycleEnd

	err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("expected nil error from Stop(), got %v", err)
	}
}

func TestOrchestratorCallbacks(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	var startCalled, endCalled atomic.Int32
	cfg := OrchestratorConfig{
		SpecDir:   filepath.Join(dir, "specs"),
		MaxCycles: 1,
		RalphStarter: func(ctx context.Context, specPath string) error {
			return nil
		},
		OnCycleStart: func(cycleNum int, specPath string) {
			startCalled.Add(1)
		},
		OnCycleEnd: func(cycleNum int, result CycleResult) {
			endCalled.Add(1)
		},
	}

	o := NewOrchestrator(mod, cfg)
	o.Run(context.Background())

	if startCalled.Load() != 1 {
		t.Errorf("expected OnCycleStart called once, got %d", startCalled.Load())
	}
	if endCalled.Load() != 1 {
		t.Errorf("expected OnCycleEnd called once, got %d", endCalled.Load())
	}
}

func TestOrchestratorResults(t *testing.T) {
	dir := t.TempDir()
	rmPath := writeTestRoadmapFile(t, dir)

	mod := NewModule(CycleConfig{
		RoadmapPath: rmPath,
		ScanRepos:   []string{"test/repo"},
	})

	cfg := OrchestratorConfig{
		SpecDir:   filepath.Join(dir, "specs"),
		MaxCycles: 2,
		RalphStarter: func(ctx context.Context, specPath string) error {
			return nil
		},
	}

	o := NewOrchestrator(mod, cfg)
	o.Run(context.Background())

	results := o.Results()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.CycleNum != i+1 {
			t.Errorf("result %d: expected cycle %d, got %d", i, i+1, r.CycleNum)
		}
	}
}

func TestOrchestratorAlreadyRunning(t *testing.T) {
	mod := NewModule(CycleConfig{})
	o := NewOrchestrator(mod, OrchestratorConfig{MaxCycles: 1})

	o.mu.Lock()
	o.running = true
	o.mu.Unlock()

	err := o.Run(context.Background())
	if err == nil {
		t.Error("expected error when already running")
	}
}

// writeTestRoadmapFile creates a minimal roadmap JSON for testing.
func writeTestRoadmapFile(t *testing.T, dir string) string {
	t.Helper()
	rm := &roadmap.Roadmap{
		Title: "test",
		Phases: []roadmap.Phase{
			{
				ID:     "p1",
				Name:   "Phase 1",
				Status: roadmap.PhaseStatusActive,
				Items: []roadmap.WorkItem{
					{ID: "item-1", Description: "Test item", Status: roadmap.ItemStatusPlanned},
				},
			},
		},
	}
	path := filepath.Join(dir, "ROADMAP.md")
	if err := roadmap.SaveRoadmap(path, rm); err != nil {
		t.Fatal(err)
	}
	return path
}
