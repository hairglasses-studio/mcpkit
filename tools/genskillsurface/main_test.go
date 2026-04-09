package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckOutputs(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(repoRoot, ".agents", "skills", "surface.yaml")); err != nil {
		t.Fatalf("surface config missing: %v", err)
	}
	if err := checkOutputs(repoRoot); err != nil {
		t.Fatalf("checkOutputs() error: %v", err)
	}
}

func TestFrontDoorDocsPresent(t *testing.T) {
	cfg, err := loadSurfaceConfig(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("loadSurfaceConfig() error: %v", err)
	}
	if len(cfg.FrontDoors) == 0 {
		t.Fatal("expected at least one front door")
	}
}

func TestGenerateOutputsCopiesReferenceTrees(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	files, err := generateOutputs(repoRoot)
	if err != nil {
		t.Fatalf("generateOutputs() error: %v", err)
	}

	required := map[string]bool{
		filepath.Join(".claude", "skills", "mcpkit", "references", "workflows.md"):                 false,
		filepath.Join(".claude", "skills", "mcpkit", "references", "legacy-aliases.md"):            false,
		filepath.Join(".claude", "skills", "fix-issue", "references", "workflows.md"):              false,
		filepath.Join(".claude", "skills", "mcp-tool-scaffold", "references", "legacy-aliases.md"): false,
	}
	for _, f := range files {
		if _, ok := required[f.Path]; ok {
			required[f.Path] = true
		}
		if f.Path == filepath.Join("docs", "SKILL-FRONT-DOORS.md") && !strings.Contains(string(f.Content), "Framework Mapping") {
			t.Fatalf("front door markdown missing expected content")
		}
	}
	for path, found := range required {
		if !found {
			t.Fatalf("expected generated output %q", path)
		}
	}
}
