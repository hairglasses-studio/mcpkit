package fleetinventory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scoreFixture: "clean" repo (well-described unique tools, manifest active),
// "messy" repo (violations, scaffold + duplicate names, archived lifecycle),
// with a shared duplicate tool name across both.
func scoreFixture(t *testing.T) string {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[
			{"name":"clean","scope":"active_operator","lifecycle":"active"},
			{"name":"messy","scope":"compatibility_only","lifecycle":"archived"}]}`,
		"clean/.git/HEAD": "x",
		"clean/AGENTS.md": "c",
		"clean/tools.go": `package c

import "x/mcp"

func f() {
	_ = mcp.NewTool("clean_alpha", mcp.WithDescription("Does alpha."))
	_ = mcp.NewTool("clean_beta", mcp.WithDescription("Does beta."))
	_ = mcp.NewTool("shared_tool", mcp.WithDescription("Shared."))
}
`,
		"clean/.well-known/mcp.json": `{"tool_count":3}`,
		"messy/.git/HEAD":            "x",
		"messy/GEMINI.md":            "retired",
		"messy/tools.go": `package m

import "x/mcp"

func f() {
	_ = mcp.NewTool("shared_tool")
	_ = mcp.NewTool("shared_tool")
	_ = mcp.NewTool("echo")
	_ = mcp.NewTool("messy_thing")
}
`,
	})
	return root
}

func TestScoreEndToEnd(t *testing.T) {
	root := scoreFixture(t)
	rep, err := Scan(context.Background(), root, ScanOptions{Score: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Scoring == nil {
		t.Fatal("scoring missing")
	}
	// Score:true without IncludeSurfaces must strip detail from output.
	for _, r := range rep.Repos {
		if r.SurfaceDetail != nil {
			t.Errorf("%s: surface detail leaked", r.Repo)
		}
	}

	byName := map[string]RepoScore{}
	for _, s := range rep.Scoring.Repos {
		byName[s.Repo] = s
	}
	clean, messy := byName["clean"], byName["messy"]

	if clean.Composite == nil || messy.Composite == nil {
		t.Fatalf("composites missing: %+v %+v", clean, messy)
	}
	if *clean.Composite <= *messy.Composite {
		t.Errorf("clean (%.1f) should outscore messy (%.1f)", *clean.Composite, *messy.Composite)
	}
	if clean.Dims.DescriptionCoverage == nil || *clean.Dims.DescriptionCoverage != 100 {
		t.Errorf("clean desc = %v", clean.Dims.DescriptionCoverage)
	}
	if messy.Dims.ViolationBurden >= 100 {
		t.Errorf("messy violations not penalized: %d", messy.Dims.ViolationBurden)
	}
	// clean declared 3 tools, scanned 3 -> perfect declared_gap.
	if clean.Dims.DeclaredGap == nil || *clean.Dims.DeclaredGap != 100 {
		t.Errorf("clean declared_gap = %v", clean.Dims.DeclaredGap)
	}
	// messy has no .well-known -> unmeasured.
	if messy.Dims.DeclaredGap != nil {
		t.Errorf("messy declared_gap should be nil, got %v", messy.Dims.DeclaredGap)
	}
	// Impact: archived+compat (0.2*0.3) < active (1.0).
	if messy.ImpactWeight >= clean.ImpactWeight {
		t.Errorf("impact weights wrong: messy %.2f clean %.2f", messy.ImpactWeight, clean.ImpactWeight)
	}
	// Priority ordering: first entry has the highest roadmap priority.
	if len(rep.Scoring.Repos) > 1 {
		p0, p1 := rep.Scoring.Repos[0].RoadmapPriority, rep.Scoring.Repos[1].RoadmapPriority
		if p0 != nil && p1 != nil && *p0 < *p1 {
			t.Error("scoreboard not priority-sorted")
		}
	}

	// Namespace view: shared_tool spans both repos under prefix "shared".
	foundShared := false
	for _, ns := range rep.Scoring.Namespaces {
		if ns.Namespace == "shared" {
			foundShared = true
			if ns.RepoSpan != 2 || !ns.CrossRepoDuplicate {
				t.Errorf("shared namespace = %+v", ns)
			}
		}
	}
	if !foundShared {
		t.Error("shared namespace not detected")
	}

	md := RenderMarkdown(rep)
	for _, want := range []string{"## Quality Scoreboard", "| clean |", "| messy |"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestTruncatedWalkCapsConfidence(t *testing.T) {
	root := scoreFixture(t)
	// Force truncation via a tiny MaxFiles.
	rep, err := Scan(context.Background(), root, ScanOptions{Score: true, Walk: WalkOptions{MaxFiles: 2}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range rep.Scoring.Repos {
		if s.DataConfidence > 0.5 {
			t.Errorf("%s: truncated repo confidence %.2f > 0.5", s.Repo, s.DataConfidence)
		}
	}
	_ = os.Remove(filepath.Join(root, "unused"))
}
