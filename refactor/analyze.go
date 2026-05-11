package refactor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type AnalyzeScriptInput struct {
	ScriptPath string `json:"script_path" jsonschema:"required,description=Absolute path to the script file."`
}

type AnalyzeScriptOutput struct {
	ScriptPath            string `json:"script_path"`
	LineCount             int    `json:"line_count"`
	UsesNetwork           bool   `json:"uses_network"`
	UsesConcurrency       bool   `json:"uses_concurrency"`
	UsesSystem            bool   `json:"uses_system"`
	RecommendedLanguage   string `json:"recommended_language"`
	Reason                string `json:"reason"`
	SuggestedArchitecture string `json:"suggested_architecture"`
}

func analyzeScriptToolDef() registry.ToolDefinition {
	td := handler.TypedHandler[AnalyzeScriptInput, AnalyzeScriptOutput](
		"refactor_analyze_script",
		"Analyze a script file to recommend refactoring approaches and target languages.",
		handleAnalyzeScript,
	)
	td.Category = "refactor"
	return td
}

func handleAnalyzeScript(ctx context.Context, args AnalyzeScriptInput) (AnalyzeScriptOutput, error) {
	var out AnalyzeScriptOutput
	out.ScriptPath = args.ScriptPath

	data, err := os.ReadFile(args.ScriptPath)
	if err != nil {
		return out, fmt.Errorf("failed to read script: %w", err)
	}
	content := string(data)

	// Basic analysis heuristics
	lines := strings.Split(content, "\n")
	out.LineCount = len(lines)

	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "curl") || strings.Contains(lower, "wget") || strings.Contains(lower, "requests.") || strings.Contains(lower, "rclone") {
			out.UsesNetwork = true
		}
		if strings.Contains(lower, "&") || strings.Contains(lower, "wait") || strings.Contains(lower, "multiprocessing") || strings.Contains(lower, "threading") {
			out.UsesConcurrency = true
		}
		if strings.Contains(lower, "os.") || strings.Contains(lower, "subprocess") || strings.Contains(lower, "systemctl") || strings.Contains(lower, "hyprctl") {
			out.UsesSystem = true
		}
	}

	out.RecommendedLanguage = "Go"
	out.Reason = "Go is preferred for network-heavy, concurrent orchestration tasks in the hairglasses-studio fleet."

	if out.UsesSystem && !out.UsesNetwork && out.LineCount < 100 {
		out.RecommendedLanguage = "Rust"
		out.Reason = "Rust is preferred for single-binary fast startup, CLI launchers, and system-level bindings."
	}

	out.SuggestedArchitecture = "Use `mcpkit/workflow` for orchestrated stages if Go, or a standard Rust CLI if Rust."

	return out, nil
}
