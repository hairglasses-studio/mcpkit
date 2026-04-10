package rdcycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// PlanInput is the input for the rdcycle_plan tool.
type PlanInput struct {
	RoadmapPath string   `json:"roadmap_path,omitempty" jsonschema:"description=Path to roadmap JSON (default: config path)"`
	ActionItems []string `json:"action_items,omitempty" jsonschema:"description=Action items from scan to incorporate into planning suggestions"`
}

// PlanOutput is the output of the rdcycle_plan tool.
type PlanOutput struct {
	NextPhase   *roadmap.Phase     `json:"next_phase"`
	ReadyItems  []roadmap.WorkItem `json:"ready_items"`
	GapCount    int                `json:"gap_count"`
	Suggestions []string           `json:"suggestions"`
	ArtifactID  string             `json:"artifact_id"`
}

func (m *Module) planTool() registry.ToolDefinition {
	desc := "Load the roadmap, identify the next incomplete phase, run gap analysis, and draft planning suggestions. " +
		"Optionally pass action_items from a prior scan to incorporate ecosystem signals into suggestions. " +
		"Returns the next phase, ready-to-start items, gap count, and a list of prioritised suggestions. " +
		"Use roadmap_path to override the module-configured roadmap file."

	td := handler.TypedHandler[PlanInput, PlanOutput](
		"rdcycle_plan",
		desc,
		m.handlePlan,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.Complexity = registry.ComplexityModerate
	return td
}

func (m *Module) handlePlan(ctx context.Context, input PlanInput) (PlanOutput, error) {
	path := m.config.RoadmapPath
	if input.RoadmapPath != "" {
		path = input.RoadmapPath
	}
	if path == "" {
		path = "ROADMAP.md"
	}

	rm, err := roadmap.LoadRoadmap(path)
	if err != nil {
		return PlanOutput{}, fmt.Errorf("rdcycle_plan: load roadmap: %w", err)
	}

	nextPhase := roadmap.NextPhase(rm)
	gaps := roadmap.GapAnalysis(rm)
	var readyItems []roadmap.WorkItem
	if nextPhase != nil {
		readyItems = roadmap.ReadyItems(nextPhase)
	}

	suggestions := buildSuggestions(nextPhase, gaps, readyItems, input.ActionItems)

	output := PlanOutput{
		NextPhase:   nextPhase,
		ReadyItems:  readyItems,
		GapCount:    len(gaps),
		Suggestions: suggestions,
		ArtifactID:  fmt.Sprintf("plan-%d", time.Now().UnixNano()),
	}

	_ = m.store.Save(Artifact{
		ID:        output.ArtifactID,
		Type:      "plan",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Content: map[string]any{
			"roadmap_path": path,
			"gap_count":    output.GapCount,
			"ready_count":  len(readyItems),
			"suggestions":  suggestions,
			"action_items": input.ActionItems,
		},
	})

	return output, nil
}

// buildSuggestions produces an ordered list of planning suggestions.
func buildSuggestions(phase *roadmap.Phase, gaps []roadmap.WorkItem, ready []roadmap.WorkItem, actionItems []string) []string {
	var suggestions []string

	if phase == nil {
		suggestions = append(suggestions, "All phases are complete — consider defining a new phase to address remaining gaps or ecosystem changes.")
	} else {
		suggestions = append(suggestions, fmt.Sprintf("Focus on phase %q (%s): %d item(s) ready to start.", phase.Name, phase.ID, len(ready)))
	}

	if len(ready) > 0 {
		names := make([]string, 0, len(ready))
		for _, item := range ready {
			names = append(names, fmt.Sprintf("%s (%s)", item.ID, truncate(item.Description, 60)))
		}
		suggestions = append(suggestions, "Ready items: "+strings.Join(names, "; "))
	}

	if len(gaps) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Gap analysis: %d planned item(s) remain across all phases.", len(gaps)))
	}

	// Fold action items from the scan into suggestions.
	for _, ai := range actionItems {
		suggestions = append(suggestions, "Ecosystem signal: "+ai)
	}

	return suggestions
}

// truncate shortens a string to maxLen, appending "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
