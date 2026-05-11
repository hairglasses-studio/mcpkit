package main

import (
	"os"
	"path/filepath"
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
