package rdcycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleReport_EmptyTitle(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	_, err := m.handleReport(context.Background(), ReportInput{
		Summary: "test",
	})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestHandleReport_EmptySummary(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	_, err := m.handleReport(context.Background(), ReportInput{
		Title: "test",
	})
	if err == nil {
		t.Error("expected error for empty summary")
	}
}

func TestHandleReport_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewModule(CycleConfig{GitRoot: dir})

	out, err := m.handleReport(context.Background(), ReportInput{
		Title:       "SDK Landscape",
		Summary:     "Found 3 new SDK releases.",
		ActionItems: []string{"Upgrade mcp-go", "Review go-sdk changes"},
		OutputDir:   dir,
	})
	if err != nil {
		t.Fatalf("handleReport: %v", err)
	}
	if !out.Written {
		t.Error("expected written=true")
	}

	// Verify file contents
	data, err := os.ReadFile(out.FilePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Research: SDK Landscape") {
		t.Error("missing title in report")
	}
	if !strings.Contains(content, "Found 3 new SDK releases.") {
		t.Error("missing summary in report")
	}
	if !strings.Contains(content, "- Upgrade mcp-go") {
		t.Error("missing action item in report")
	}
}

func TestHandleReport_FilenameFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewModule(CycleConfig{GitRoot: dir})

	out, err := m.handleReport(context.Background(), ReportInput{
		Title:     "MCP Evolution",
		Summary:   "Summary text.",
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("handleReport: %v", err)
	}

	expected := filepath.Join(dir, "RESEARCH-MCP-EVOLUTION.md")
	if out.FilePath != expected {
		t.Errorf("filePath = %q; want %q", out.FilePath, expected)
	}
}

func TestHandleReport_ActionItemsRendering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewModule(CycleConfig{})

	items := []string{"Fix bug A", "Add feature B", "Update docs"}
	out, err := m.handleReport(context.Background(), ReportInput{
		Title:       "Action Test",
		Summary:     "Summary with actions.",
		ActionItems: items,
		OutputDir:   dir,
	})
	if err != nil {
		t.Fatalf("handleReport: %v", err)
	}

	data, err := os.ReadFile(out.FilePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "## Action Items") {
		t.Error("missing Action Items section header")
	}
	for _, item := range items {
		if !strings.Contains(content, "- "+item) {
			t.Errorf("missing action item %q in report", item)
		}
	}
}

func TestHandleReport_NoActionItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewModule(CycleConfig{})

	out, err := m.handleReport(context.Background(), ReportInput{
		Title:     "No Actions",
		Summary:   "Plain summary.",
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("handleReport: %v", err)
	}

	data, err := os.ReadFile(out.FilePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "## Action Items") {
		t.Error("should not have Action Items section when none provided")
	}
}

func TestHandleReport_DefaultOutputDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// GitRoot is set — OutputDir is empty, so GitRoot should be used.
	m := NewModule(CycleConfig{GitRoot: dir})

	out, err := m.handleReport(context.Background(), ReportInput{
		Title:   "Default Dir",
		Summary: "Summary.",
	})
	if err != nil {
		t.Fatalf("handleReport: %v", err)
	}

	expected := filepath.Join(dir, "RESEARCH-DEFAULT-DIR.md")
	if out.FilePath != expected {
		t.Errorf("filePath = %q; want %q", out.FilePath, expected)
	}
}
