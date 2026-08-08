package fleetinventory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fixture lays out a workspace: manifest, catalog, and two repos — one
// compliant, one violating (retired mirror + local codex profiles).
func fixture(t *testing.T) string {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json":          `{"version":1,"repos":[{"name":"good"},{"name":"bad"},{"name":"ghost"}]}`,
		"docs/inventory/repo-catalog.json": `{"repos":[{"name":"good"},{"name":"phantom"}]}`,
		"good/.git/HEAD":                   "ref: refs/heads/main",
		"good/AGENTS.md":                   "# canonical",
		"good/CLAUDE.md":                   "mirror",
		"good/ROADMAP.md":                  "plan",
		"good/.claude/settings.json":       `{}`,
		"good/.mcp.json":                   `{"mcpServers":{"a":{},"b":{}}}`,
		"good/.agents/skills/surface.yaml": "skills: []",
		"good/.agents/skills/s1/SKILL.md":  "skill",
		"good/.claude/skills/s1/SKILL.md":  "generated",
		"good/.agents/agents/x/agent.md":   "agent",
		"good/tools.go":                    "package good\n\nimport \"x/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"good_tool\") }\n",
		"bad/.git/HEAD":                    "ref: refs/heads/main",
		"bad/GEMINI.md":                    "retired mirror",
		"bad/.codex/config.toml":           "[profiles.fast]\nmodel = \"x\"\n",
		"bad/main.go":                      "package main\n\nimport \"flag\"\n\nfunc main() { _ = flag.NewFlagSet(\"serve\", 0) }\n",
	})
	return root
}

func TestScanEndToEnd(t *testing.T) {
	root := fixture(t)
	fixed := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	rep, err := Scan(context.Background(), root, ScanOptions{Now: func() time.Time { return fixed }})
	if err != nil {
		t.Fatal(err)
	}
	if rep.GeneratedOn != "2026-08-08T12:00:00Z" {
		t.Errorf("generated_on = %s", rep.GeneratedOn)
	}
	if len(rep.Repos) != 3 { // good, bad, ghost (missing dir recorded, not fatal)
		t.Fatalf("repos = %d, want 3", len(rep.Repos))
	}

	byName := map[string]RepoReport{}
	for _, r := range rep.Repos {
		byName[r.Repo] = r
	}

	good := byName["good"]
	if good.Parity.AgentsMD != 1 || good.Parity.ClaudeMD != 1 || !good.Parity.HasRoadmap || !good.Parity.RootClaudeSet {
		t.Errorf("good parity wrong: %+v", good.Parity)
	}
	if good.Parity.MCPServerCount != 2 {
		t.Errorf("mcp server count = %d, want 2", good.Parity.MCPServerCount)
	}
	if good.Parity.CanonicalSkills != 1 || good.Parity.GeneratedClaudeSkills != 1 {
		t.Errorf("skill counts wrong: %+v", good.Parity)
	}
	if good.Parity.ClaudeAgents != 1 {
		t.Errorf("claude agents = %d", good.Parity.ClaudeAgents)
	}
	if len(good.Parity.Violations) != 0 {
		t.Errorf("good has violations: %v", good.Parity.Violations)
	}
	if good.Surfaces["mcp_tool"] != 1 {
		t.Errorf("good mcp_tool = %d", good.Surfaces["mcp_tool"])
	}

	bad := byName["bad"]
	wantViolations := []string{"retired mirror present: GEMINI.md", "repo-local .codex/config.toml defines [profiles.*]", "missing AGENTS.md"}
	if len(bad.Parity.Violations) != len(wantViolations) {
		t.Fatalf("bad violations = %v, want %v", bad.Parity.Violations, wantViolations)
	}
	for _, want := range wantViolations {
		found := false
		for _, v := range bad.Parity.Violations {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing violation %q in %v", want, bad.Parity.Violations)
		}
	}
	if bad.Surfaces["cli_command"] != 1 {
		t.Errorf("bad cli_command = %d", bad.Surfaces["cli_command"])
	}
	if rep.ViolationCount != len(wantViolations) {
		t.Errorf("violation_count = %d", rep.ViolationCount)
	}

	ghost := byName["ghost"]
	if len(ghost.WalkErrors) == 0 {
		t.Error("ghost repo missing-dir not recorded")
	}

	// Drift: ghost is manifest-only; phantom is catalog-only; nothing on-disk-only.
	if len(rep.Drift.ManifestOnly) != 1 || rep.Drift.ManifestOnly[0] != "ghost" {
		t.Errorf("manifest_only = %v", rep.Drift.ManifestOnly)
	}
	if len(rep.Drift.CatalogOnly) != 1 || rep.Drift.CatalogOnly[0] != "phantom" {
		t.Errorf("catalog_only = %v", rep.Drift.CatalogOnly)
	}
	if len(rep.Drift.OnDiskOnly) != 0 {
		t.Errorf("on_disk_only = %v", rep.Drift.OnDiskOnly)
	}

	// The report must round-trip as JSON (it is the MCP tool output).
	if _, err := json.Marshal(rep); err != nil {
		t.Fatalf("report not marshalable: %v", err)
	}
}

func TestWalkRepoBounds(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{}
	for i := 0; i < 20; i++ {
		files[filepath.Join("d", "f"+string(rune('a'+i))+".txt")] = "x"
	}
	files["node_modules/dep/index.js"] = "skip"
	files[".hidden/secret.txt"] = "skip"
	files[".claude/settings.json"] = "keep"
	writeFiles(t, root, files)

	idx := WalkRepo(context.Background(), root, "r", WalkOptions{MaxFiles: 10})
	if !idx.Truncated {
		t.Error("MaxFiles did not truncate")
	}
	if len(idx.Paths) != 10 {
		t.Errorf("paths = %d, want 10", len(idx.Paths))
	}

	full := WalkRepo(context.Background(), root, "r", WalkOptions{})
	for _, p := range full.Paths {
		if strings.HasPrefix(p, "node_modules/") || strings.HasPrefix(p, ".hidden/") {
			t.Errorf("pruned path leaked: %s", p)
		}
	}
	if !full.Has(".claude/settings.json") {
		t.Error(".claude not exempted from hidden-dir pruning")
	}
}

func TestRenderMarkdownSections(t *testing.T) {
	root := fixture(t)
	rep, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(rep)
	for _, want := range []string{"# Fleet Inventory", "| good |", "retired mirror present: GEMINI.md", "## Drift", "ghost", "phantom"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}
