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
