package rdcycle

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/ralph"
)

// CandidateTask is a potential task for inclusion in a cycle.
type CandidateTask struct {
	ID          string
	Description string
	Source      string // "roadmap", "improvement", "scaffold"
	Priority    int    // lower = higher priority
	DependsOn   []string
	Complexity  string // "simple", "moderate", "complex"
}

// TaskSource provides candidate tasks from a specific source.
type TaskSource interface {
	Fetch(ctx context.Context) ([]CandidateTask, error)
}

// SynthesisConfig controls the synthesis process.
type SynthesisConfig struct {
	CycleName   string
	RoadmapPath string
	Strategy    CycleStrategy
	MaxTasks    int // max implementation tasks to include (0 = unlimited)
}

// Synthesizer combines multiple task sources with learning signals to produce
// an optimal ralph.Spec for the next cycle.
type Synthesizer struct {
	sources  []TaskSource
	learning *LearningEngine
}

// NewSynthesizer creates a Synthesizer with the given sources and learning engine.
func NewSynthesizer(sources []TaskSource, learning *LearningEngine) *Synthesizer {
	return &Synthesizer{
		sources:  sources,
		learning: learning,
	}
}

// Synthesize produces a ralph.Spec by fetching candidates from all sources,
// filtering by avoid patterns, applying strategy constraints, and building a DAG.
func (s *Synthesizer) Synthesize(ctx context.Context, cfg SynthesisConfig) (ralph.Spec, error) {
	if cfg.CycleName == "" {
		return ralph.Spec{}, fmt.Errorf("rdcycle: cycle_name is required for synthesis")
	}

	// Fetch from all sources concurrently.
	type fetchResult struct {
		tasks []CandidateTask
		err   error
	}
	results := make([]fetchResult, len(s.sources))
	var wg sync.WaitGroup
	for i, src := range s.sources {
		wg.Add(1)
		go func(idx int, source TaskSource) {
			defer wg.Done()
			tasks, err := source.Fetch(ctx)
			results[idx] = fetchResult{tasks: tasks, err: err}
		}(i, src)
	}
	wg.Wait()

	// Collect all candidates.
	var candidates []CandidateTask
	for _, r := range results {
		if r.err != nil {
			continue // skip failed sources
		}
		candidates = append(candidates, r.tasks...)
	}

	// Filter by avoid patterns.
	avoidPatterns := s.learning.AvoidPatterns(10)
	if len(avoidPatterns) > 0 {
		candidates = filterAvoidPatterns(candidates, avoidPatterns)
	}

	// Apply strategy filter.
	candidates = filterByStrategy(candidates, cfg.Strategy)

	// Sort by priority (lower = higher priority).
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})

	// Apply max tasks limit.
	if cfg.MaxTasks > 0 && len(candidates) > cfg.MaxTasks {
		candidates = candidates[:cfg.MaxTasks]
	}

	// Apply task mutations from learning engine.
	mutations := s.learning.TaskMutations()
	candidates = applyMutations(candidates, mutations)

	// Build the spec with scaffolding tasks.
	spec := buildSynthesizedSpec(cfg, candidates, s.learning)

	return spec, nil
}

// filterAvoidPatterns removes candidates whose description matches avoid patterns.
func filterAvoidPatterns(candidates []CandidateTask, patterns []string) []CandidateTask {
	var filtered []CandidateTask
	for _, c := range candidates {
		avoided := false
		lower := strings.ToLower(c.Description)
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				avoided = true
				break
			}
		}
		if !avoided {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// filterByStrategy removes candidates that don't match the active strategy.
func filterByStrategy(candidates []CandidateTask, strategy CycleStrategy) []CandidateTask {
	switch strategy {
	case StrategyMaintenance:
		var filtered []CandidateTask
		for _, c := range candidates {
			if c.Source != "roadmap" {
				filtered = append(filtered, c)
			}
		}
		return filtered
	case StrategyEcosystem:
		var filtered []CandidateTask
		for _, c := range candidates {
			if c.Source != "roadmap" || c.Complexity == "simple" {
				filtered = append(filtered, c)
			}
		}
		return filtered
	case StrategyRecovery:
		var filtered []CandidateTask
		for _, c := range candidates {
			if c.Source == "improvement" || c.Source == "scaffold" {
				filtered = append(filtered, c)
			}
		}
		return filtered
	case StrategyMetaImprove:
		var filtered []CandidateTask
		for _, c := range candidates {
			if c.Source == "improvement" {
				filtered = append(filtered, c)
			}
		}
		return filtered
	default: // StrategyFull
		return candidates
	}
}

// applyMutations modifies the candidate list based on learning mutations.
func applyMutations(candidates []CandidateTask, mutations []TaskMutation) []CandidateTask {
	for _, m := range mutations {
		switch m.Action {
		case "add_verify":
			var updated []CandidateTask
			for _, c := range candidates {
				updated = append(updated, c)
				if c.ID == m.TaskID {
					updated = append(updated, CandidateTask{
						ID:          c.ID + "_pre_verify",
						Description: fmt.Sprintf("Pre-verify after %s: run build + vet before continuing.", c.ID),
						Source:      "improvement",
						Priority:    c.Priority + 1,
						DependsOn:   []string{c.ID},
						Complexity:  "simple",
					})
				}
			}
			candidates = updated
		case "meta_improve":
			hasImprove := false
			for _, c := range candidates {
				if c.ID == "self_improve" {
					hasImprove = true
					break
				}
			}
			if !hasImprove {
				candidates = append(candidates, CandidateTask{
					ID:          "self_improve",
					Description: "Run rdcycle_improve to analyze accumulated notes and apply recommendations.",
					Source:      "improvement",
					Priority:    50,
					Complexity:  "simple",
				})
			}
		}
	}
	return candidates
}

// buildSynthesizedSpec constructs a ralph.Spec from candidates with scaffolding tasks.
func buildSynthesizedSpec(cfg SynthesisConfig, candidates []CandidateTask, learning *LearningEngine) ralph.Spec {
	sinceDate := time.Now().UTC().Format(time.RFC3339)
	description := fmt.Sprintf("Autonomous R&D cycle '%s' (strategy: %s, since: %s).",
		cfg.CycleName, cfg.Strategy, sinceDate)

	// Inject lessons summary.
	costTrend := learning.CostTrend()
	avoidPatterns := learning.AvoidPatterns(10)
	if costTrend != "stable" || len(avoidPatterns) > 0 {
		description += "\n\nLearning signals:"
		if costTrend != "stable" {
			description += fmt.Sprintf("\n- Cost trend: %s", costTrend)
		}
		for _, p := range avoidPatterns {
			description += fmt.Sprintf("\n- Avoid: %s", p)
		}
	}

	var tasks []ralph.Task

	// Prepend scan task for strategies that include ecosystem scanning.
	if cfg.Strategy == StrategyFull || cfg.Strategy == StrategyEcosystem {
		tasks = append(tasks, ralph.Task{
			ID:          "scan",
			Description: fmt.Sprintf("Run rdcycle_scan to check ecosystem activity since %s.", sinceDate),
		})
	}

	// Prepend plan task for strategies that include planning.
	if cfg.Strategy == StrategyFull || cfg.Strategy == StrategyEcosystem {
		tasks = append(tasks, ralph.Task{
			ID:          "plan",
			Description: fmt.Sprintf("Run rdcycle_plan with scan results and roadmap at %s.", cfg.RoadmapPath),
			DependsOn:   []string{"scan"},
		})
	}

	// Add candidate tasks as implementation tasks.
	lastImplID := "plan"
	if cfg.Strategy == StrategyRecovery || cfg.Strategy == StrategyMaintenance {
		lastImplID = ""
	}
	for _, c := range candidates {
		deps := c.DependsOn
		if len(deps) == 0 && lastImplID != "" {
			deps = []string{lastImplID}
		}
		tasks = append(tasks, ralph.Task{
			ID:          c.ID,
			Description: c.Description,
			DependsOn:   deps,
		})
		lastImplID = c.ID
	}

	// Append scaffolding: verify, reflect, schedule.
	verifyDeps := []string{}
	if lastImplID != "" {
		verifyDeps = []string{lastImplID}
	}
	tasks = append(tasks, ralph.Task{
		ID:          "verify",
		Description: "Run rdcycle_verify to execute make check. Fix issues and re-verify if needed.",
		DependsOn:   verifyDeps,
	})
	tasks = append(tasks, ralph.Task{
		ID:          "reflect",
		Description: "Record improvement notes for this cycle using rdcycle_notes.",
		DependsOn:   []string{"verify"},
	})
	tasks = append(tasks, ralph.Task{
		ID:          "schedule",
		Description: "Run rdcycle_schedule to write the next cycle's spec.",
		DependsOn:   []string{"reflect"},
	})

	return ralph.Spec{
		Name:        fmt.Sprintf("R&D Cycle: %s", cfg.CycleName),
		Description: description,
		Completion:  "All planned work items are implemented, tests pass, and roadmap is updated.",
		Tasks:       tasks,
	}
}
