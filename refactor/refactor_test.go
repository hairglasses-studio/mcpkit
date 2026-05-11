package refactor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverScripts(t *testing.T) {
	// Create a temporary workspace
	tempDir := t.TempDir()

	// Create valid scripts
	valid1 := filepath.Join(tempDir, "script1.sh")
	valid2 := filepath.Join(tempDir, "script2.py")
	os.WriteFile(valid1, []byte("#!/bin/bash"), 0755)
	os.WriteFile(valid2, []byte("print('hello')"), 0644)

	// Create ignored directory and script
	nodeModulesDir := filepath.Join(tempDir, "node_modules")
	os.Mkdir(nodeModulesDir, 0755)
	ignored1 := filepath.Join(nodeModulesDir, "ignored.sh")
	os.WriteFile(ignored1, []byte("#!/bin/bash"), 0755)

	// Call handler
	out, err := handleDiscoverScripts(context.Background(), DiscoverScriptsInput{WorkspacePath: tempDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Count != 2 {
		t.Errorf("expected 2 scripts, got %d", out.Count)
	}

	foundValid1 := false
	foundIgnored := false
	for _, s := range out.Scripts {
		if strings.HasSuffix(s, "script1.sh") {
			foundValid1 = true
		}
		if strings.Contains(s, "node_modules") {
			foundIgnored = true
		}
	}

	if !foundValid1 {
		t.Errorf("expected to find script1.sh")
	}
	if foundIgnored {
		t.Errorf("expected to ignore node_modules scripts")
	}
}

func TestAnalyzeScript_GoRecommendation(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "network.sh")

	content := `#!/bin/bash
curl -s https://api.example.com/data
wait
`
	os.WriteFile(scriptPath, []byte(content), 0755)

	out, err := handleAnalyzeScript(context.Background(), AnalyzeScriptInput{ScriptPath: scriptPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !out.UsesNetwork {
		t.Errorf("expected UsesNetwork to be true")
	}
	if out.RecommendedLanguage != "Go" {
		t.Errorf("expected Go recommendation, got %s", out.RecommendedLanguage)
	}
}

func TestAnalyzeScript_RustRecommendation(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "system.sh")

	content := `#!/bin/bash
hyprctl dispatch exec kitty
`
	os.WriteFile(scriptPath, []byte(content), 0755)

	out, err := handleAnalyzeScript(context.Background(), AnalyzeScriptInput{ScriptPath: scriptPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.UsesNetwork {
		t.Errorf("expected UsesNetwork to be false")
	}
	if !out.UsesSystem {
		t.Errorf("expected UsesSystem to be true")
	}
	if out.RecommendedLanguage != "Rust" {
		t.Errorf("expected Rust recommendation, got %s", out.RecommendedLanguage)
	}
}

func TestPlanConversion(t *testing.T) {
	out, err := handlePlanConversion(context.Background(), PlanConversionInput{
		ScriptPath:     "/test/script.sh",
		TargetLanguage: "Go",
		Stages:         "download, analyze, sync",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.MarkdownPlan, "graph.AddNode(\"download\", handleDownload)") {
		t.Errorf("missing download node in plan")
	}
	if !strings.Contains(out.MarkdownPlan, "graph.AddEdge(\"download\", \"analyze\")") {
		t.Errorf("missing edge in plan")
	}
	if !strings.Contains(out.MarkdownPlan, "graph.SetStart(\"download\")") {
		t.Errorf("missing start node in plan")
	}
}
