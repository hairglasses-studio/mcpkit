package rdcycle

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/roadmap"
	"github.com/hairglasses-studio/mcpkit/workflow"
)

// NewRDCycleGraph builds a workflow graph for the R&D cycle:
// scan → plan → gate → implement (fork) → verify → gate_quality → END
// The quality gate loops back to implement on failure.
func NewRDCycleGraph(cfg CycleConfig) (*workflow.Graph, error) {
	if cfg.RoadmapPath == "" {
		return nil, fmt.Errorf("rdcycle: roadmap path is required")
	}

	g := workflow.NewGraph()

	// Node: scan — stores scan summary in state
	if err := g.AddNode("scan", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		s.Data["scan_repos"] = cfg.ScanRepos
		s.Data["scan_since"] = cfg.DateRange
		s.Data["scan_complete"] = true
		return s, nil
	}); err != nil {
		return nil, err
	}

	// Node: plan — loads roadmap, finds next phase and ready items
	if err := g.AddNode("plan", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		rm, err := roadmap.LoadRoadmap(cfg.RoadmapPath)
		if err != nil {
			return s, fmt.Errorf("plan: load roadmap: %w", err)
		}
		phase := roadmap.NextPhase(rm)
		if phase == nil {
			s.Data["plan_empty"] = true
			return s, nil
		}
		s.Data["plan_phase_id"] = phase.ID
		s.Data["plan_phase_name"] = phase.Name
		s.Data["plan_ready_count"] = len(roadmap.ReadyItems(phase))
		s.Data["plan_gap_count"] = len(roadmap.GapAnalysis(rm))
		return s, nil
	}); err != nil {
		return nil, err
	}

	// Node: gate — checks if there's work to do
	if err := g.AddNode("gate", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		return s, nil
	}); err != nil {
		return nil, err
	}

	// Node: implement — compensable node that rolls back on failure.
	// Uses AddCompensableNode so that if verify fails, the compensation function
	// can clean up (e.g., git checkout) orphaned code.
	implementForward := func(ctx context.Context, s workflow.State) (workflow.State, error) {
		s.Data["implement_complete"] = true
		return s, nil
	}
	implementCompensate := func(ctx context.Context, s workflow.State) error {
		// In real usage, this would run `git checkout -- .` to revert changes.
		// For now, mark that compensation was triggered.
		s.Data["implement_compensated"] = true
		return nil
	}
	if err := g.AddCompensableNode("implement", implementForward, implementCompensate); err != nil {
		return nil, err
	}

	// Node: verify — runs quality checks
	if err := g.AddNode("verify", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		// The actual verify logic would run make check
		s.Data["verify_passed"] = true
		return s, nil
	}); err != nil {
		return nil, err
	}

	// Node: gate_quality — routes based on verify result
	if err := g.AddNode("gate_quality", func(ctx context.Context, s workflow.State) (workflow.State, error) {
		retries, _ := workflow.Get[int](s, "quality_retries")
		s.Data["quality_retries"] = retries + 1
		return s, nil
	}); err != nil {
		return nil, err
	}

	// Edges: scan → plan → gate
	if err := g.AddEdge("scan", "plan"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("plan", "gate"); err != nil {
		return nil, err
	}

	// Gate: if no work to do, go to END; otherwise implement
	if err := g.AddConditionalEdge("gate", func(s workflow.State) string {
		if empty, _ := workflow.Get[bool](s, "plan_empty"); empty {
			return workflow.EndNode
		}
		return "implement"
	}); err != nil {
		return nil, err
	}

	// implement → verify → gate_quality
	if err := g.AddEdge("implement", "verify"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("verify", "gate_quality"); err != nil {
		return nil, err
	}

	// Quality gate: if verify passed → END, else retry implement (max 3)
	if err := g.AddConditionalEdge("gate_quality", func(s workflow.State) string {
		if passed, _ := workflow.Get[bool](s, "verify_passed"); passed {
			return workflow.EndNode
		}
		retries, _ := workflow.Get[int](s, "quality_retries")
		if retries >= 3 {
			return workflow.EndNode // give up after 3 retries
		}
		return "implement"
	}); err != nil {
		return nil, err
	}

	if err := g.SetStart("scan"); err != nil {
		return nil, err
	}

	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("rdcycle: graph validation: %w", err)
	}

	return g, nil
}
