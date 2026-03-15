package rdcycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ReportInput is the input for the rdcycle_report tool.
type ReportInput struct {
	Title       string   `json:"title" jsonschema:"required,description=Report title (used in filename as RESEARCH-<TITLE>.md)"`
	Summary     string   `json:"summary" jsonschema:"required,description=Report summary text"`
	ActionItems []string `json:"action_items,omitempty" jsonschema:"description=Action items to include in the report"`
	OutputDir   string   `json:"output_dir,omitempty" jsonschema:"description=Directory for report output (default: git root)"`
}

// ReportOutput is the output of the rdcycle_report tool.
type ReportOutput struct {
	FilePath string `json:"file_path"`
	Written  bool   `json:"written"`
}

func (m *Module) reportTool() registry.ToolDefinition {
	desc := "Generate a RESEARCH-*.md report from scan/analysis data. " +
		"Creates a markdown file following the existing RESEARCH-*.md format."

	td := handler.TypedHandler[ReportInput, ReportOutput](
		"rdcycle_report",
		desc,
		m.handleReport,
	)
	td.IsWrite = true
	return td
}

func (m *Module) handleReport(_ context.Context, input ReportInput) (ReportOutput, error) {
	if input.Title == "" {
		return ReportOutput{}, fmt.Errorf("title is required")
	}
	if input.Summary == "" {
		return ReportOutput{}, fmt.Errorf("summary is required")
	}

	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = m.config.GitRoot
		if outputDir == "" {
			outputDir = "."
		}
	}

	// Build filename from title
	safeName := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.Title), " ", "-"))
	filename := fmt.Sprintf("RESEARCH-%s.md", safeName)
	filePath := filepath.Join(outputDir, filename)

	// Build markdown content
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Research: %s\n\n", input.Title))
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("## Summary\n\n")
	b.WriteString(input.Summary)
	b.WriteString("\n")

	if len(input.ActionItems) > 0 {
		b.WriteString("\n## Action Items\n\n")
		for _, item := range input.ActionItems {
			b.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}

	if err := os.WriteFile(filePath, []byte(b.String()), 0644); err != nil {
		return ReportOutput{}, fmt.Errorf("write report: %w", err)
	}

	return ReportOutput{
		FilePath: filePath,
		Written:  true,
	}, nil
}
