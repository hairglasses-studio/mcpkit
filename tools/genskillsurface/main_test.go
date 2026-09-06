package main

import (
	"io/fs"
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

func TestBrokenSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "projection")
	if err := os.Symlink(filepath.Join(root, "missing"), link); err != nil {
		t.Fatalf("create broken symlink: %v", err)
	}
	if !brokenSymlink(link) {
		t.Fatal("brokenSymlink() = false for a dangling projection")
	}

	dir := filepath.Join(root, "present")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if brokenSymlink(dir) {
		t.Fatal("brokenSymlink() = true for a real directory")
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

// TestCheckOutputsSkipsAbsentClaudeProjection pins the two halves of the
// `.claude` projection rule. `.claude/` is gitignored in this repo, so a fresh
// clone has none of it: checkOutputs must not fail on projected files that were
// never checked in (the state mcpkit's skill-surface-check gate landed in once
// the repo's dangling `.claude` symlink was replaced with a real directory).
// It must still report drift for a projected file that IS present and wrong,
// otherwise the skip would make the whole gate vacuous.
func TestCheckOutputsSkipsAbsentClaudeProjection(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	files, err := generateOutputs(repoRoot)
	if err != nil {
		t.Fatalf("generateOutputs() error: %v", err)
	}

	claudePrefix := ".claude" + string(filepath.Separator)
	tmp := t.TempDir()
	copyTree(t, filepath.Join(repoRoot, ".agents"), filepath.Join(tmp, ".agents"))

	var claudeOutput *docFile
	for i, f := range files {
		if strings.HasPrefix(filepath.Clean(f.Path), claudePrefix) {
			if claudeOutput == nil {
				claudeOutput = &files[i]
			}
			continue
		}
		writeFile(t, filepath.Join(tmp, f.Path), f.Content)
	}
	if claudeOutput == nil {
		t.Fatal("no .claude projection outputs generated; test cannot cover the skip")
	}

	// Half 1: projection entirely absent.
	if err := checkOutputs(tmp); err != nil {
		t.Fatalf("checkOutputs() with no .claude projection: %v", err)
	}

	// Half 2: projection present but drifted.
	writeFile(t, filepath.Join(tmp, claudeOutput.Path), append([]byte("drifted\n"), claudeOutput.Content...))
	if err := checkOutputs(tmp); err == nil {
		t.Fatalf("checkOutputs() accepted drifted %s; the absent-file skip made the gate vacuous", claudeOutput.Path)
	}
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	}); err != nil {
		t.Fatalf("copyTree(%s): %v", src, err)
	}
}
