package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	dir := t.TempDir()
	if got := detectLanguage(dir); got != "unknown" {
		t.Fatalf("unknown detect = %q", got)
	}

	goDir := filepath.Join(dir, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module x\n\ngo 1.26.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectLanguage(goDir); got != "go" {
		t.Fatalf("go detect = %q", got)
	}
}
