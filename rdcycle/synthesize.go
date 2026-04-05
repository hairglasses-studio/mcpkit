package rdcycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hairglasses-studio/mcpkit/ralph"
)

// TaskSynthesizer converts PlanOutput into ralph.Spec files with per-item tasks.
type TaskSynthesizer struct {
	SpecDir string
}

// SynthesizeSpec converts a PlanOutput and lessons learned into a ralph.Spec.
// Each ReadyItem from the plan becomes an implement task. Scan action items
// become investigation tasks. The standard DAG structure is preserved.
func (ts *TaskSynthesizer) SynthesizeSpec(plan PlanOutput, cycleName string, lessons []string) (ralph.Spec, error) {
	if cycleName == "" {
		return ralph.Spec{}, fmt.Errorf("synthesize: cycle_name is required")
	}

	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("Autonomous R&D cycle: %s.", cycleName))
	if plan.NextPhase != nil {
		desc.WriteString(fmt.Sprintf(" Focus: phase %q.", plan.NextPhase.Name))
	}
	if len(lessons) > 0 {
		desc.WriteString("\n\nLessons from previous cycles:\n")
		for _, l := range lessons {
			desc.WriteString("- " + l + "\n")
		}
	}

	spec := ralph.Spec{
		Name:        fmt.Sprintf("R&D Cycle: %s", cycleName),
		Description: desc.String(),
		Completion:  "All planned work items are implemented, tests pass, and roadmap is updated.",
	}

	// Fixed tasks: scan and plan.
	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "scan",
		Description: "Run rdcycle_scan to check ecosystem activity.",
	})
	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "plan",
		Description: "Run rdcycle_plan with scan results.",
		DependsOn:   []string{"scan"},
	})

	// Dynamic implement tasks from ready items.
	var implementIDs []string
	for _, item := range plan.ReadyItems {
		taskID := "implement_" + sanitizeTaskID(item.ID)
		implementIDs = append(implementIDs, taskID)
		desc := fmt.Sprintf("Implement %s: %s", item.ID, item.Description)
		if item.Package != "" {
			desc += fmt.Sprintf(" (package: %s)", item.Package)
		}
		spec.Tasks = append(spec.Tasks, ralph.Task{
			ID:          taskID,
			Description: desc,
			DependsOn:   []string{"plan"},
		})
	}

	// Investigation tasks from scan action items in suggestions.
	for i, suggestion := range plan.Suggestions {
		if !strings.HasPrefix(suggestion, "Ecosystem signal: ") {
			continue
		}
		taskID := fmt.Sprintf("investigate_%d", i)
		implementIDs = append(implementIDs, taskID)
		spec.Tasks = append(spec.Tasks, ralph.Task{
			ID:          taskID,
			Description: strings.TrimPrefix(suggestion, "Ecosystem signal: "),
			DependsOn:   []string{"plan"},
		})
	}

	// If no implement tasks, add a generic one.
	if len(implementIDs) == 0 {
		implementIDs = []string{"implement"}
		spec.Tasks = append(spec.Tasks, ralph.Task{
			ID:          "implement",
			Description: "Implement changes based on plan suggestions.",
			DependsOn:   []string{"plan"},
		})
	}

	// verify depends on all implement tasks.
	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "verify",
		Description: "Run rdcycle_verify to execute make check. Fix issues and re-verify if needed.",
		DependsOn:   implementIDs,
	})

	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "reflect",
		Description: "Record improvement notes using rdcycle_notes: what worked, what failed, wasted iterations, cost, and suggestions.",
		DependsOn:   []string{"verify"},
	})

	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "report",
		Description: "Run rdcycle_report to generate research reports. Run rdcycle_commit to save changes.",
		DependsOn:   []string{"reflect"},
	})

	spec.Tasks = append(spec.Tasks, ralph.Task{
		ID:          "schedule",
		Description: "Run rdcycle_schedule to write the next cycle's spec.",
		DependsOn:   []string{"report"},
	})

	return spec, nil
}

// WriteSpec writes a ralph.Spec to disk as JSON. Returns the file path.
func (ts *TaskSynthesizer) WriteSpec(spec ralph.Spec, cycleName string) (string, error) {
	dir := ts.SpecDir
	if dir == "" {
		dir = filepath.Join("rdcycle", "specs")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("synthesize: create dir: %w", err)
	}

	filename := sanitizeTaskID(cycleName) + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("synthesize: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("synthesize: write: %w", err)
	}
	return path, nil
}

// sanitizeTaskID makes a string safe for use as a task ID.
func sanitizeTaskID(s string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", ".", "_")
	return strings.ToLower(r.Replace(s))
}
