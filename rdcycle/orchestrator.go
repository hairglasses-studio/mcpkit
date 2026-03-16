package rdcycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/ralph"
)

// CycleResult holds the outcome of a single orchestrated cycle.
type CycleResult struct {
	CycleNum   int       `json:"cycle_num"`
	SpecPath   string    `json:"spec_path"`
	ScanOutput ScanOutput `json:"scan_output"`
	PlanOutput PlanOutput `json:"plan_output"`
	Progress   bool      `json:"progress"` // true if tasks were completed
	Cost       float64   `json:"cost"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// OrchestratorConfig configures the perpetual cycle orchestrator.
type OrchestratorConfig struct {
	ArtifactStore ArtifactStore
	Breaker       *CircuitBreaker
	Governor      *CostVelocityGovernor
	SpecDir       string
	MaxCycles     int // 0 = unlimited
	ImproveCadence int // run improve every N cycles (default 5)
	OnCycleStart  func(cycleNum int, specPath string)
	OnCycleEnd    func(cycleNum int, result CycleResult)
	RalphStarter  func(ctx context.Context, specPath string) error
	// CostReader returns the cumulative dollar cost so far.
	// Used to compute per-cycle cost deltas for the governor.
	CostReader    func() float64
}

// Orchestrator runs the perpetual R&D cycle loop.
type Orchestrator struct {
	mu           sync.Mutex
	mod          *Module
	cfg          OrchestratorConfig
	synth        *TaskSynthesizer
	adaptiveSynth *Synthesizer
	running      bool
	stopCh       chan struct{}
	cycleNum     int
	results      []CycleResult
}

// NewOrchestrator creates a new orchestrator bound to the given module.
func NewOrchestrator(mod *Module, cfg OrchestratorConfig) *Orchestrator {
	if cfg.ImproveCadence <= 0 {
		cfg.ImproveCadence = 5
	}
	specDir := cfg.SpecDir
	if specDir == "" {
		specDir = "rdcycle/specs"
	}
	o := &Orchestrator{
		mod:   mod,
		cfg:   cfg,
		synth: &TaskSynthesizer{SpecDir: specDir},
	}

	// Build adaptive synthesizer if sources are available.
	if cfg.ArtifactStore != nil {
		notesPath := "rdcycle/notes/improvement_log.json"
		learning := NewLearningEngine(notesPath)
		var sources []TaskSource
		if mod.config.RoadmapPath != "" {
			sources = append(sources, NewRoadmapSource(mod.config.RoadmapPath))
		}
		sources = append(sources, NewImprovementSource(notesPath))
		o.adaptiveSynth = NewSynthesizer(sources, learning)
	}

	return o
}

// Run executes the perpetual cycle loop until context is cancelled, the breaker
// opens, the governor halts, or MaxCycles is reached.
func (o *Orchestrator) Run(ctx context.Context) error {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator: already running")
	}
	o.running = true
	o.stopCh = make(chan struct{})
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.running = false
		o.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopCh:
			return nil
		default:
		}

		result, err := o.RunOneCycle(ctx)
		if err != nil {
			// Record the failed result.
			result.Error = err.Error()
		}

		o.mu.Lock()
		o.results = append(o.results, result)
		maxCycles := o.cfg.MaxCycles
		cycleNum := o.cycleNum
		o.mu.Unlock()

		if o.cfg.OnCycleEnd != nil {
			o.cfg.OnCycleEnd(cycleNum, result)
		}

		// Check termination conditions.
		if maxCycles > 0 && cycleNum >= maxCycles {
			return nil
		}

		// Governor halt check.
		if o.cfg.Governor != nil && o.cfg.Governor.ShouldHalt() {
			return fmt.Errorf("orchestrator: governor halted after cycle %d", cycleNum)
		}
	}
}

// RunOneCycle executes a single R&D cycle: scan → plan → synthesize → ralph → notes → improve.
func (o *Orchestrator) RunOneCycle(ctx context.Context) (CycleResult, error) {
	o.mu.Lock()
	o.cycleNum++
	cycleNum := o.cycleNum
	o.mu.Unlock()

	result := CycleResult{
		CycleNum:  cycleNum,
		StartedAt: time.Now(),
	}

	// 1. Circuit breaker check.
	if o.cfg.Breaker != nil && !o.cfg.Breaker.CanExecute() {
		result.FinishedAt = time.Now()
		return result, fmt.Errorf("circuit breaker open (state=%s)", o.cfg.Breaker.State())
	}

	// 2. Governor check.
	if o.cfg.Governor != nil && o.cfg.Governor.ShouldHalt() {
		result.FinishedAt = time.Now()
		return result, fmt.Errorf("governor halted: cost or productivity threshold exceeded")
	}

	// 3. Scan.
	scanOut, err := o.mod.HandleScan(ctx, ScanInput{})
	if err != nil {
		result.FinishedAt = time.Now()
		o.recordBreakerResult(false, true)
		return result, fmt.Errorf("scan: %w", err)
	}
	result.ScanOutput = scanOut

	// 4. Plan.
	planOut, err := o.mod.HandlePlan(ctx, PlanInput{
		ActionItems: scanOut.ActionItems,
	})
	if err != nil {
		result.FinishedAt = time.Now()
		o.recordBreakerResult(false, true)
		return result, fmt.Errorf("plan: %w", err)
	}
	result.PlanOutput = planOut

	// 5. Synthesize spec — prefer adaptive synthesizer, fall back to legacy.
	var spec ralph.Spec
	var specPath string
	cycleName := fmt.Sprintf("cycle-%d", cycleNum)

	if o.adaptiveSynth != nil {
		notes, _ := LoadNotes("rdcycle/notes/improvement_log.json")
		strategy := SelectStrategy(notes, 0, 1.0)
		synthSpec, synthErr := o.adaptiveSynth.Synthesize(ctx, SynthesisConfig{
			CycleName:   cycleName,
			RoadmapPath: o.mod.config.RoadmapPath,
			Strategy:    strategy,
		})
		if synthErr == nil {
			spec = synthSpec
			var writeErr error
			specPath, writeErr = o.synth.WriteSpec(spec, cycleName)
			if writeErr != nil {
				// fall through to legacy path
				spec = ralph.Spec{}
			}
		}
	}

	if spec.Name == "" {
		// Legacy path.
		lessons := o.gatherLessons()
		var err error
		spec, err = o.synth.SynthesizeSpec(planOut, cycleName, lessons)
		if err != nil {
			result.FinishedAt = time.Now()
			o.recordBreakerResult(false, true)
			return result, fmt.Errorf("synthesize: %w", err)
		}
		specPath, err = o.synth.WriteSpec(spec, cycleName)
		if err != nil {
			result.FinishedAt = time.Now()
			o.recordBreakerResult(false, true)
			return result, fmt.Errorf("write spec: %w", err)
		}
	}
	result.SpecPath = specPath

	if o.cfg.OnCycleStart != nil {
		o.cfg.OnCycleStart(cycleNum, specPath)
	}

	// 6. Run ralph loop (blocks until completion).
	// Snapshot cost before ralph starts so we can compute the delta.
	costBefore := 0.0
	if o.cfg.CostReader != nil {
		costBefore = o.cfg.CostReader()
	}

	if o.cfg.RalphStarter != nil {
		if err := o.cfg.RalphStarter(ctx, specPath); err != nil {
			cycleCost := 0.0
			if o.cfg.CostReader != nil {
				cycleCost = o.cfg.CostReader() - costBefore
			}
			result.Cost = cycleCost
			result.FinishedAt = time.Now()
			result.Progress = false
			o.recordBreakerResult(false, false)
			o.recordGovernor(cycleNum, cycleCost, false)
			return result, fmt.Errorf("ralph: %w", err)
		}
	}

	cycleCost := 0.0
	if o.cfg.CostReader != nil {
		cycleCost = o.cfg.CostReader() - costBefore
	}
	result.Cost = cycleCost
	result.Progress = true

	// 7. Record retrospective notes.
	_, _ = o.mod.HandleNotes(ctx, NotesInput{
		CycleID:     cycleName,
		CycleNumber: cycleNum,
		WhatWorked:  []string{fmt.Sprintf("Completed cycle %d", cycleNum)},
		WhatFailed:  []string{},
		Suggestions: []string{},
	})

	// 8. Every Nth cycle: meta-improvement.
	if cycleNum%o.cfg.ImproveCadence == 0 {
		_, _ = o.mod.HandleImprove(ctx, ImproveInput{})
	}

	// 9. Record in breaker and governor.
	o.recordBreakerResult(true, false)
	o.recordGovernor(cycleNum, cycleCost, true)

	result.FinishedAt = time.Now()
	return result, nil
}

// Stop signals the orchestrator to stop after the current cycle.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.running && o.stopCh != nil {
		select {
		case <-o.stopCh:
		default:
			close(o.stopCh)
		}
	}
}

// Running returns whether the orchestrator is currently running.
func (o *Orchestrator) Running() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

// CycleNum returns the current cycle number.
func (o *Orchestrator) CycleNum() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.cycleNum
}

// Results returns a copy of all cycle results.
func (o *Orchestrator) Results() []CycleResult {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]CycleResult, len(o.results))
	copy(out, o.results)
	return out
}

func (o *Orchestrator) recordBreakerResult(progress bool, errRepeated bool) {
	if o.cfg.Breaker != nil {
		o.cfg.Breaker.RecordResult(progress, errRepeated)
	}
}

func (o *Orchestrator) recordGovernor(cycleNum int, cost float64, productive bool) {
	if o.cfg.Governor != nil {
		o.cfg.Governor.Record(CycleRecord{
			CycleNum:   cycleNum,
			Cost:       cost,
			Productive: productive,
		})
	}
}

func (o *Orchestrator) gatherLessons() []string {
	notes, _ := LoadNotes("rdcycle/notes/improvement_log.json")
	if len(notes) == 0 {
		return nil
	}

	lastN := notes
	if len(lastN) > 3 {
		lastN = lastN[len(lastN)-3:]
	}

	var lessons []string
	for _, n := range lastN {
		lessons = append(lessons, n.Suggestions...)
		for _, f := range n.WhatFailed {
			lessons = append(lessons, "Avoid: "+f)
		}
	}
	return lessons
}
