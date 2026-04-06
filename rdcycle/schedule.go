package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ScheduleInput is the input for the rdcycle_schedule tool.
type ScheduleInput struct {
	CycleName      string `json:"cycle_name" jsonschema:"required,description=Name for the next cycle"`
	OutputPath     string `json:"output_path,omitempty" jsonschema:"description=Path for the output spec file (default: rdcycle/specs/next_cycle.json)"`
	RoadmapPath    string `json:"roadmap_path,omitempty" jsonschema:"description=Roadmap path for template (default: config path)"`
	PlanArtifactID string `json:"plan_artifact_id,omitempty" jsonschema:"description=Artifact ID from rdcycle_plan to synthesize spec from (uses static template if absent)"`
}

// ScheduleOutput is the output of the rdcycle_schedule tool.
type ScheduleOutput struct {
	SpecPath  string `json:"spec_path"`
	Written   bool   `json:"written"`
	SinceDate string `json:"since_date"`
}

func (m *Module) scheduleTool() registry.ToolDefinition {
	desc := "Write a Ralph spec file for the next R&D cycle, parameterized with the current " +
		"date as since_date. Uses the rd_cycle.json template format."

	td := handler.TypedHandler[ScheduleInput, ScheduleOutput](
		"rdcycle_schedule",
		desc,
		m.handleSchedule,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.IsWrite = true
	return td
}

func (m *Module) handleSchedule(ctx context.Context, input ScheduleInput) (ScheduleOutput, error) {
	if input.CycleName == "" {
		return ScheduleOutput{}, fmt.Errorf("cycle_name is required")
	}

	// Adaptive synthesis: when a Synthesizer is configured, use it instead of the
	// hardcoded template. The Synthesizer fetches tasks from roadmap + improvement
	// sources, filters by avoid patterns and strategy, and builds an optimal DAG.
	if m.config.Synthesizer != nil {
		roadmapPath := input.RoadmapPath
		if roadmapPath == "" {
			roadmapPath = m.config.RoadmapPath
			if roadmapPath == "" {
				roadmapPath = "roadmap.json"
			}
		}

		// Determine strategy from improvement history.
		notesPath := filepath.Join("rdcycle", "notes", "improvement_log.json")
		notes, _ := LoadNotes(notesPath)
		consecutiveSuccess := ConsecutiveSuccesses(notes)
		spent := 0.0
		if m.costReader != nil {
			spent = m.costReader()
		}
		budgetPct := BudgetPct(m.config.TotalBudget, spent)
		strategy := SelectStrategy(notes, consecutiveSuccess, budgetPct)

		spec, err := m.config.Synthesizer.Synthesize(ctx, SynthesisConfig{
			CycleName:   input.CycleName,
			RoadmapPath: roadmapPath,
			Strategy:    strategy,
		})
		if err == nil {
			outputPath := resolveOutputPath(input)
			dir := filepath.Dir(outputPath)
			os.MkdirAll(dir, 0755)
			data, merr := json.MarshalIndent(spec, "", "  ")
			if merr == nil {
				if werr := os.WriteFile(outputPath, data, 0644); werr == nil {
					return ScheduleOutput{
						SpecPath:  strings.ReplaceAll(outputPath, "\\", "/"),
						Written:   true,
						SinceDate: time.Now().UTC().Format(time.RFC3339),
					}, nil
				}
			}
		}
		// Fall through to legacy template on synthesis error.
	}

	// When a plan artifact is referenced, synthesize a dynamic spec.
	if input.PlanArtifactID != "" {
		if art, ok := m.store.Get(input.PlanArtifactID); ok {
			plan, err := planOutputFromArtifact(art)
			if err == nil {
				notesPath := filepath.Join("rdcycle", "notes", "improvement_log.json")
				notes, _ := LoadNotes(notesPath)
				var lessons []string
				lastN := notes
				if len(lastN) > 3 {
					lastN = lastN[len(lastN)-3:]
				}
				for _, n := range lastN {
					lessons = append(lessons, n.Suggestions...)
					for _, f := range n.WhatFailed {
						lessons = append(lessons, "Avoid: "+f)
					}
				}

				ts := &TaskSynthesizer{SpecDir: filepath.Dir(resolveOutputPath(input))}
				spec, err := ts.SynthesizeSpec(plan, input.CycleName, lessons)
				if err == nil {
					path, err := ts.WriteSpec(spec, input.CycleName)
					if err == nil {
						return ScheduleOutput{
							SpecPath:  strings.ReplaceAll(path, "\\", "/"),
							Written:   true,
							SinceDate: time.Now().UTC().Format(time.RFC3339),
						}, nil
					}
				}
			}
		}
		// Fall through to static template if artifact not found or synthesis fails.
	}

	sinceDate := time.Now().UTC().Format(time.RFC3339)
	roadmapPath := input.RoadmapPath
	if roadmapPath == "" {
		roadmapPath = m.config.RoadmapPath
		if roadmapPath == "" {
			roadmapPath = "roadmap.json"
		}
	}

	// Build the spec using the template structure
	spec := map[string]any{
		"name":        fmt.Sprintf("R&D Cycle: %s", input.CycleName),
		"description": fmt.Sprintf("Autonomous R&D cycle scanning MCP ecosystem changes since %s, planning roadmap updates, implementing changes, and verifying quality.", sinceDate),
		"completion":  "All planned work items are implemented, tests pass, and roadmap is updated.",
		"tasks": []map[string]any{
			{
				"id":          "scan",
				"description": fmt.Sprintf("Run rdcycle_scan to check ecosystem activity since %s.", sinceDate),
			},
			{
				"id":          "plan",
				"description": fmt.Sprintf("Run rdcycle_plan with scan results and roadmap at %s.", roadmapPath),
				"depends_on":  []string{"scan"},
			},
			{
				"id":          "implement",
				"description": "For each ready work item from the plan, implement the changes.",
				"depends_on":  []string{"plan"},
			},
			{
				"id":          "verify",
				"description": "Run rdcycle_verify to execute make check. Fix issues and re-verify if needed.",
				"depends_on":  []string{"implement"},
			},
			{
				"id":          "reflect",
				"description": "Record improvement notes for this cycle using rdcycle_notes: what worked, what failed, wasted iterations, cost, and suggestions for next cycle.",
				"depends_on":  []string{"verify"},
			},
			{
				"id":          "report",
				"description": "Run rdcycle_report to generate research reports. Run rdcycle_commit to save changes.",
				"depends_on":  []string{"reflect"},
			},
			{
				"id":          "schedule",
				"description": "Run rdcycle_schedule to write the next cycle's spec.",
				"depends_on":  []string{"report"},
			},
		},
	}

	// Load improvement notes and inject lessons learned.
	notesPath := filepath.Join("rdcycle", "notes", "improvement_log.json")
	notes, _ := LoadNotes(notesPath)
	if len(notes) > 0 {
		// Inject last 3 notes as lessons learned context.
		lastN := notes
		if len(lastN) > 3 {
			lastN = lastN[len(lastN)-3:]
		}
		var lessons []string
		for _, n := range lastN {
			for _, s := range n.Suggestions {
				lessons = append(lessons, s)
			}
			for _, f := range n.WhatFailed {
				lessons = append(lessons, "Avoid: "+f)
			}
		}
		if len(lessons) > 0 {
			var desc strings.Builder
			desc.WriteString(spec["description"].(string))
			desc.WriteString("\n\nLessons from previous cycles:\n")
			for _, l := range lessons {
				desc.WriteString("- " + l + "\n")
			}
			spec["description"] = desc.String()
		}

		// Every 10 cycles, add self_improve task.
		if len(notes)%10 == 0 {
			tasks := spec["tasks"].([]map[string]any)
			improveTask := map[string]any{
				"id":          "self_improve",
				"description": "Run rdcycle_improve to analyze accumulated notes and apply recommendations.",
				"depends_on":  []string{"reflect"},
			}
			for i, task := range tasks {
				if task["id"] == "report" {
					tasks[i]["depends_on"] = []string{"self_improve"}
					break
				}
			}
			var newTasks []map[string]any
			for _, task := range tasks {
				if task["id"] == "report" {
					newTasks = append(newTasks, improveTask)
				}
				newTasks = append(newTasks, task)
			}
			spec["tasks"] = newTasks
		}
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return ScheduleOutput{}, fmt.Errorf("marshal spec: %w", err)
	}

	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join("rdcycle", "specs", "next_cycle.json")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ScheduleOutput{}, fmt.Errorf("create dir: %w", err)
	}

	// Atomic write
	tmp, err := os.CreateTemp(dir, ".schedule-*.tmp")
	if err != nil {
		return ScheduleOutput{}, fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return ScheduleOutput{}, fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()
	if err := os.Rename(tmpName, outputPath); err != nil {
		os.Remove(tmpName)
		return ScheduleOutput{}, fmt.Errorf("rename: %w", err)
	}

	// Normalize the output path separators
	outputPath = strings.ReplaceAll(outputPath, "\\", "/")

	return ScheduleOutput{
		SpecPath:  outputPath,
		Written:   true,
		SinceDate: sinceDate,
	}, nil
}

// resolveOutputPath resolves the output path from input, applying defaults.
func resolveOutputPath(input ScheduleInput) string {
	if input.OutputPath != "" {
		return input.OutputPath
	}
	return filepath.Join("rdcycle", "specs", "next_cycle.json")
}

// planOutputFromArtifact reconstructs a PlanOutput from a stored artifact's content.
func planOutputFromArtifact(art Artifact) (PlanOutput, error) {
	var plan PlanOutput

	if suggestions, ok := art.Content["suggestions"].([]any); ok {
		for _, s := range suggestions {
			if str, ok := s.(string); ok {
				plan.Suggestions = append(plan.Suggestions, str)
			}
		}
	}

	if gapCount, ok := art.Content["gap_count"].(float64); ok {
		plan.GapCount = int(gapCount)
	}

	return plan, nil
}
