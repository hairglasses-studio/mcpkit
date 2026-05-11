package refactor

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type PlanConversionInput struct {
	ScriptPath     string `json:"script_path" jsonschema:"required,description=Absolute path to the script being migrated."`
	TargetLanguage string `json:"target_language" jsonschema:"required,description=The target language ('Go' or 'Rust')."`
	Stages         string `json:"stages" jsonschema:"required,description=Comma-separated list of workflow stages (e.g., 'download,analyze,sync')."`
}

type PlanConversionOutput struct {
	MarkdownPlan string `json:"markdown_plan"`
}

func planConversionToolDef() registry.ToolDefinition {
	td := handler.TypedHandler[PlanConversionInput, PlanConversionOutput](
		"refactor_plan_conversion",
		"Generate a structured mcpkit/workflow or Rust CLI conversion plan for a legacy script.",
		handlePlanConversion,
	)
	td.Category = "refactor"
	return td
}

func handlePlanConversion(ctx context.Context, args PlanConversionInput) (PlanConversionOutput, error) {
	var plan string
	var out PlanConversionOutput

	plan += fmt.Sprintf("# Conversion Plan: %s to %s\n\n", args.ScriptPath, args.TargetLanguage)

	switch args.TargetLanguage {
	case "Go":
		plan += "## Architecture: Go + mcpkit/workflow\n"
		plan += "We will use the `mcpkit/workflow` package to define a resilient execution graph.\n\n"

		plan += "### Graph Definition (Stub)\n"
		plan += "```go\n"
		plan += "graph := workflow.NewGraph()\n"

		stages := strings.Split(args.Stages, ",")
		for i := range stages {
			stages[i] = strings.TrimSpace(stages[i])
		}

		var validStages []string
		for _, stage := range stages {
			if stage != "" {
				validStages = append(validStages, stage)
				plan += fmt.Sprintf("graph.AddNode(\"%s\", handle%s)\n", stage, titleCase(stage))
			}
		}

		for i := 0; i < len(validStages)-1; i++ {
			plan += fmt.Sprintf("graph.AddEdge(\"%s\", \"%s\")\n", validStages[i], validStages[i+1])
		}

		if len(validStages) > 0 {
			plan += fmt.Sprintf("graph.SetStart(\"%s\")\n", validStages[0])
		}
		plan += "```\n"
	case "Rust":
		plan += "## Architecture: Rust CLI\n"
		plan += "We will use a standard Cargo workspace binary for zero-dependency execution.\n\n"
		plan += "### Key Crates\n"
		plan += "- `clap` for argument parsing.\n"
		plan += "- `tokio` if async is required.\n"
		plan += "- `anyhow` for robust error handling.\n"
	default:
		return out, fmt.Errorf("unsupported target language: %s", args.TargetLanguage)
	}

	plan += "\n## Next Steps\n"
	plan += "1. Scaffold the project directory.\n"
	plan += "2. Migrate the core logic function-by-function.\n"
	plan += "3. Add unit tests and verify dry-runs against the legacy script.\n"

	out.MarkdownPlan = plan
	return out, nil
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
