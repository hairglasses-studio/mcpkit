package fleetinventory

import (
	"context"
	"testing"
)

func checkFixture(t *testing.T, gemini bool) string {
	root := t.TempDir()
	tools := `package s

import "x/mcp"

func f() {
	_ = mcp.NewTool("a", mcp.WithDescription("Tool a."))
	_ = mcp.NewTool("b", mcp.WithDescription("Tool b."))
}
`
	files := map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[{"name":"s","scope":"active_operator","lifecycle":"active"}]}`,
		"s/.git/HEAD":             "x",
		"s/AGENTS.md":             "a",
		"s/go.mod":                "module s\nrequire github.com/mark3labs/mcp-go v0.57.0\n",
		"s/tools.go":              tools,
	}
	if gemini {
		files["s/GEMINI.md"] = "retired mirror"
	}
	writeFiles(t, root, files)
	return root
}

func TestBaselineAndCheckPass(t *testing.T) {
	root := checkFixture(t, false)
	rep, err := Scan(context.Background(), root, ScanOptions{Score: true})
	if err != nil {
		t.Fatal(err)
	}
	base := BaselineFromReport(rep)
	if _, ok := base.Repos["s"]; !ok {
		t.Fatal("baseline missing repo s")
	}
	// re-scan same tree, check against its own baseline -> pass
	rep2, _ := Scan(context.Background(), root, ScanOptions{Score: true})
	res := Check(rep2, base, 0)
	if !res.Passed || len(res.Regressions) != 0 {
		t.Errorf("identical rescan should pass: %+v", res)
	}
}

func TestCheckFailsOnNewViolation(t *testing.T) {
	cleanRoot := checkFixture(t, false)
	repClean, _ := Scan(context.Background(), cleanRoot, ScanOptions{Score: true})
	base := BaselineFromReport(repClean)

	// now scan a tree that added a retired mirror (new violation)
	dirtyRoot := checkFixture(t, true)
	repDirty, _ := Scan(context.Background(), dirtyRoot, ScanOptions{Score: true})
	res := Check(repDirty, base, 0)
	if res.Passed {
		t.Fatalf("new violation should fail: %+v", res)
	}
	found := false
	for _, r := range res.Regressions {
		if contains(r, "violations") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a violations regression: %v", res.Regressions)
	}
}

func TestCheckNewRepoNotAFailure(t *testing.T) {
	root := checkFixture(t, false)
	rep, _ := Scan(context.Background(), root, ScanOptions{Score: true})
	empty := CheckBaseline{Repos: map[string]BaselineRepo{}} // baseline knows no repos
	res := Check(rep, empty, 0)
	if !res.Passed {
		t.Errorf("all-new repos should pass, not fail: %+v", res)
	}
	if len(res.NewRepos) == 0 {
		t.Error("expected new repos reported")
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	root := checkFixture(t, false)
	rep, _ := Scan(context.Background(), root, ScanOptions{Score: true})
	raw, err := BaselineFromReport(rep).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseBaseline(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Repos) != len(rep.Repos) {
		t.Errorf("round-trip repo count mismatch: %d vs %d", len(parsed.Repos), len(rep.Repos))
	}
}
