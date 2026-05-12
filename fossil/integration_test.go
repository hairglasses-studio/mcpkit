package fossil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestScanner_EndToEndWithInstalledFossil(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external fossil-mcp integration test in short mode")
	}
	if _, err := exec.LookPath("fossil-mcp"); err != nil {
		t.Skip("fossil-mcp not installed")
	}

	dir := t.TempDir()
	src := `package main

func main() { used() }

func used() int { return 1 }

func unusedOne() int {
	a := 1
	b := 2
	c := 3
	d := 4
	e := 5
	f := 6
	return a + b + c + d + e + f
}

func unusedTwo() int {
	a := 1
	b := 2
	c := 3
	d := 4
	e := 5
	f := 6
	return a + b + c + d + e + f
}

func TODOHandler() {
	// TODO: implement real logic
	panic("not implemented")
}
`
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write temp source: %v", err)
	}

	scanner := NewScanner(ScannerConfig{
		Dir:       dir,
		Timeout:   15 * time.Second,
		FossilBin: "fossil-mcp",
	})

	ctx := context.Background()

	report, err := scanner.ScanReport(ctx)
	if err != nil {
		t.Fatalf("ScanReport() error = %v", err)
	}
	if len(report.FindingsByCategory(CategoryDeadCode)) < 2 {
		t.Fatalf("expected at least 2 dead-code findings, got %d (%+v)", len(report.FindingsByCategory(CategoryDeadCode)), report.Findings)
	}

	groups, err := scanner.DetectClones(ctx, CloneOptions{MinLines: 3})
	if err != nil {
		t.Fatalf("DetectClones() error = %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least one clone group")
	}

	scaffolding, err := scanner.DetectScaffolding(ctx, ScaffoldingOptions{IncludeTODOs: true})
	if err != nil {
		t.Fatalf("DetectScaffolding() error = %v", err)
	}
	if len(scaffolding) == 0 {
		t.Fatal("expected at least one scaffolding finding")
	}
	if scaffolding[0].Category() != CategoryScaffolding {
		t.Fatalf("expected scaffolding category, got %q", scaffolding[0].Category())
	}
}
