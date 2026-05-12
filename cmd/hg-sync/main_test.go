package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectReposFiltersLifecycleAndLanguage(t *testing.T) {
	repos := []repoEntry{
		{Name: "a", Language: "Go"},
		{Name: "b", Language: "Python", GoWorkMember: true},
		{Name: "c", Language: "Go", Lifecycle: "compatibility"},
		{Name: "d", Language: "Go", Lifecycle: "deprecated"},
	}
	got := selectRepos(repos, options{})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("unexpected repos: %#v", got)
	}
}

func TestRewriteGoVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("module x\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, after, err := rewriteGoVersion(path, "1.26.1")
	if err != nil {
		t.Fatal(err)
	}
	if before != "1.24.0" || after != "1.26.1" {
		t.Fatalf("got before=%q after=%q", before, after)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "module x\n\ngo 1.26.1\n" {
		t.Fatalf("unexpected file:\n%s", string(content))
	}
}

func TestValidateManifestSchemaRequiresVersion2(t *testing.T) {
	err := validateManifestSchema(manifest{
		Version: 1,
		Repos:   []repoEntry{{Name: "repo-a"}},
	})
	if err == nil || !strings.Contains(err.Error(), "version=2") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestParseGoWorkEntries(t *testing.T) {
	dir := t.TempDir()
	goWork := "go 1.26\n\nuse (\n\t./repo-b\n\t./repo-a\n)\n"
	path := filepath.Join(dir, "go.work")
	if err := os.WriteFile(path, []byte(goWork), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseGoWorkEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"repo-a", "repo-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestValidateScriptManifestHelpersAcceptsWrapper(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "dotfiles", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"hg-go-sync.sh":                "#!/usr/bin/env bash\nexec go run \"$HOME/hairglasses-studio/mcpkit/cmd/hg-sync/main.go\" \"$@\"\n",
		"hg-new-repo.sh":               "source \"$SCRIPT_DIR/lib/hg-workspace.sh\"\n",
		"hg-onboard-repo.sh":           "source \"$SCRIPT_DIR/lib/hg-workspace.sh\"\n",
		"hg-repo-profile-sync.sh":      "source \"$SCRIPT_DIR/lib/hg-workspace.sh\"\n",
		"hg-workflow-sync.sh":          "source \"$SCRIPT_DIR/lib/hg-workspace.sh\"\n",
		"sync-standalone-mcp-repos.sh": "source \"$SCRIPT_DIR/lib/hg-workspace.sh\"\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(scriptsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateScriptManifestHelpers(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRepoPolicyRequiresOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	raw, _ := json.Marshal(map[string]any{})
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRepoPolicy(path); err == nil || !strings.Contains(err.Error(), "repo_overrides") {
		t.Fatalf("expected repo_overrides error, got %v", err)
	}
}
