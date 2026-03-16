//go:build !official_sdk

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/ralph"
	"github.com/hairglasses-studio/mcpkit/rdcycle"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/research"
	"github.com/hairglasses-studio/mcpkit/roadmap"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// RunnerConfig configures the multi-cycle runner.
type RunnerConfig struct {
	InitialSpec  string
	TemplateVars map[string]string
	GlobalBudget float64 // total $ across all cycles (default: $60)
	Duration     time.Duration
	Sampler      sampling.SamplingClient
	ModelTier    rdcycle.ModelTierConfig
	Profile      rdcycle.BudgetProfile
	RoadmapPath  string
	StatePath    string
	GitHubToken  string
	ScanRepos    []string
}

// MultiCycleRunner chains ralph.Loop R&D cycles until budget or duration is exhausted.
type MultiCycleRunner struct {
	cfg    RunnerConfig
	state  RunnerState
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewMultiCycleRunner creates a runner from the given config.
func NewMultiCycleRunner(cfg RunnerConfig) *MultiCycleRunner {
	if cfg.GlobalBudget <= 0 {
		cfg.GlobalBudget = 60.0
	}
	if cfg.Duration <= 0 {
		cfg.Duration = 12 * time.Hour
	}
	if cfg.StatePath == "" {
		cfg.StatePath = ".rdloop_state.json"
	}
	if cfg.RoadmapPath == "" {
		cfg.RoadmapPath = "roadmap.json"
	}
	if cfg.ScanRepos == nil {
		cfg.ScanRepos = []string{
			"modelcontextprotocol/specification",
			"modelcontextprotocol/go-sdk",
			"mark3labs/mcp-go",
		}
	}
	return &MultiCycleRunner{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Run executes cycles until duration, budget, or context cancellation.
func (r *MultiCycleRunner) Run(ctx context.Context) error {
	// Load existing state for resumability.
	state, err := LoadState(r.cfg.StatePath)
	if err != nil {
		return fmt.Errorf("rdloop: load state: %w", err)
	}
	r.mu.Lock()
	r.state = state
	if r.state.StartedAt.IsZero() {
		r.state.StartedAt = time.Now()
	}
	r.mu.Unlock()

	deadline := r.state.StartedAt.Add(r.cfg.Duration)
	consecutiveFails := 0

	for {
		// Check stop conditions.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.stopCh:
			log.Println("rdloop: stop signal received")
			return nil
		default:
		}

		if time.Now().After(deadline) {
			log.Println("rdloop: duration limit reached")
			return nil
		}

		r.mu.Lock()
		if r.state.TotalCost >= r.cfg.GlobalBudget {
			r.mu.Unlock()
			log.Printf("rdloop: global budget exhausted ($%.2f >= $%.2f)", r.state.TotalCost, r.cfg.GlobalBudget)
			return nil
		}
		cycleNum := r.state.CycleNumber + 1
		r.state.CycleNumber = cycleNum
		r.mu.Unlock()

		// Determine spec for this cycle.
		specFile := r.cfg.InitialSpec
		templateVars := r.cfg.TemplateVars
		if cycleNum > 1 {
			nextSpec := "rdcycle/specs/next_cycle.json"
			if _, err := os.Stat(nextSpec); err == nil {
				specFile = nextSpec
				// next_cycle.json is fully rendered, no template vars needed.
				templateVars = nil
			} else {
				// Re-use initial spec with updated vars.
				templateVars = map[string]string{
					"cycle_name":   fmt.Sprintf("auto-%d", cycleNum),
					"since_date":   time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
					"roadmap_path": r.cfg.RoadmapPath,
				}
			}
		}

		log.Printf("rdloop: === Cycle %d starting (spec: %s) ===", cycleNum, specFile)
		cycleStart := time.Now()

		summary, err := r.runOneCycle(ctx, cycleNum, specFile, templateVars)
		cycleDuration := time.Since(cycleStart)

		if err != nil {
			log.Printf("rdloop: cycle %d failed after %s: %v", cycleNum, cycleDuration, err)
			summary.Status = "failed"
		}
		summary.Duration = cycleDuration

		r.mu.Lock()
		r.state.TotalCost += summary.Cost
		r.state.TotalIterations += summary.Iters
		r.state.LastCycleAt = time.Now()
		r.state.History = append(r.state.History, summary)
		r.mu.Unlock()

		if saveErr := SaveState(r.cfg.StatePath, r.state); saveErr != nil {
			log.Printf("rdloop: warning: failed to save state: %v", saveErr)
		}

		log.Printf("rdloop: === Cycle %d done (%s, %d iters, $%.4f) ===",
			cycleNum, summary.Status, summary.Iters, summary.Cost)

		// Back off on consecutive failures to avoid tight error loops.
		if summary.Status == "failed" {
			consecutiveFails++
			if consecutiveFails >= 3 {
				log.Printf("rdloop: %d consecutive failures, stopping", consecutiveFails)
				return fmt.Errorf("rdloop: %d consecutive cycle failures", consecutiveFails)
			}
			backoff := time.Duration(consecutiveFails) * 30 * time.Second
			log.Printf("rdloop: backing off %s before next cycle", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		} else {
			consecutiveFails = 0
		}
	}
}

// Stop signals the runner to stop after the current cycle.
func (r *MultiCycleRunner) Stop() {
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

// State returns the current runner state.
func (r *MultiCycleRunner) State() RunnerState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *MultiCycleRunner) runOneCycle(ctx context.Context, cycleNum int, specFile string, templateVars map[string]string) (CycleSummary, error) {
	summary := CycleSummary{
		Number:   cycleNum,
		SpecFile: specFile,
		Status:   "completed",
	}

	// Build per-cycle finops stack.
	tracker, _, _ := rdcycle.BuildFinOpsStack(r.cfg.Profile)

	// Build tool registry with all modules.
	reg := registry.NewToolRegistry()

	researchMod := research.NewModule(research.Config{
		GitHubToken: r.cfg.GitHubToken,
	})
	reg.RegisterModule(researchMod)

	roadmapMod := roadmap.NewModule(roadmap.Config{
		RoadmapPath: r.cfg.RoadmapPath,
	})
	reg.RegisterModule(roadmapMod)

	fileMod := &ralph.FileToolModule{Root: "."}
	reg.RegisterModule(fileMod)

	rdcycleMod := rdcycle.NewModule(rdcycle.CycleConfig{
		RoadmapPath: r.cfg.RoadmapPath,
		GitRoot:     ".",
		ScanRepos:   r.cfg.ScanRepos,
	})
	reg.RegisterModule(rdcycleMod)

	// Configure ralph loop.
	progressFile := fmt.Sprintf(".rdloop_cycle_%d.progress.json", cycleNum)

	loopCfg := ralph.Config{
		SpecFile:      specFile,
		ProgressFile:  progressFile,
		ToolRegistry:  reg,
		Sampler:       r.cfg.Sampler,
		MaxIterations: r.cfg.Profile.MaxIterations,
		MaxTokens:     r.cfg.Profile.MaxTokensPerReq,
		CostTracker:   tracker,
		ForceRestart:  true,
		TemplateVars:  templateVars,
		ModelSelector: rdcycle.CombineSelectors(
			r.cfg.ModelTier.Selector(),
			rdcycle.NewCostAdapter(int64(r.cfg.Profile.MaxIterations)*int64(r.cfg.Profile.MaxTokensPerReq), r.cfg.Profile.MaxIterations),
			tracker,
		),
		HistoryWindow: 5,
		AutoVerifyLevel: ralph.AutoVerifyFull,
		ProjectRoot:   ".",
		PhaseMaxTokens: map[string]int{
			"scan": 2048, "plan": 4096, "implement": 16384,
			"verify": 2048, "reflect": 2048, "report": 4096, "schedule": 2048,
		},
		Hooks: ralph.Hooks{
			OnIterationStart: func(iter int) {
				log.Printf("  [cycle %d] iteration %d starting", cycleNum, iter)
			},
			OnIterationEnd: func(entry ralph.IterationLog) {
				taskInfo := ""
				if entry.TaskID != "" {
					taskInfo = fmt.Sprintf(" (task: %s)", entry.TaskID)
				}
				toolInfo := ""
				if len(entry.ToolCalls) > 0 {
					toolInfo = fmt.Sprintf(" tools=[%s]", strings.Join(entry.ToolCalls, ","))
				}
				log.Printf("  [cycle %d] iteration %d done%s%s: %s",
					cycleNum, entry.Iteration, taskInfo, toolInfo, truncate(entry.Result, 120))
			},
			OnTaskComplete: func(taskID string) {
				log.Printf("  [cycle %d] task %q completed", cycleNum, taskID)
				summary.Tasks = append(summary.Tasks, taskID)
			},
			OnError: func(iter int, err error) {
				log.Printf("  [cycle %d] iteration %d error: %v", cycleNum, iter, err)
			},
			OnCostUpdate: func(iter int, us finops.UsageSummary) {
				cost := estimateDollarCost(us, r.cfg.Profile)
				summary.Cost = cost
				log.Printf("  [cycle %d] cost: $%.4f (tokens: %d in, %d out)",
					cycleNum, cost, us.TotalInputTokens, us.TotalOutputTokens)
			},
		},
	}

	loop, err := ralph.NewLoop(loopCfg)
	if err != nil {
		return summary, fmt.Errorf("create loop: %w", err)
	}

	err = loop.Run(ctx)

	// Read final progress for iteration count.
	progress := loop.Status()
	summary.Iters = progress.Iteration

	// Clean up progress file after cycle.
	os.Remove(progressFile)

	return summary, err
}

// estimateDollarCost converts a UsageSummary to dollar cost using profile pricing.
func estimateDollarCost(us finops.UsageSummary, profile rdcycle.BudgetProfile) float64 {
	// Use sonnet pricing as default (most common model in the loop).
	inputRate := 0.003  // per 1K tokens
	outputRate := 0.015 // per 1K tokens
	for _, mp := range profile.ModelPricing {
		if mp.Model == "claude-sonnet-4-6" {
			inputRate = mp.InputPer1KTokens
			outputRate = mp.OutputPer1KTokens
			break
		}
	}
	return float64(us.TotalInputTokens)/1000*inputRate + float64(us.TotalOutputTokens)/1000*outputRate
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
