package fleetinventory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const vulnModulesFixture = `[
  {"path":"github.com/vuln/lib","vulns":[{"id":"GO-2026-0001","fixed":"1.4.0"},{"id":"GO-2026-0002","fixed":"1.2.0"}]},
  {"path":"github.com/safe/lib","vulns":[{"id":"GO-2026-0009","fixed":"2.0.0"}]}
]`

func writeVulnDB(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "modules.json")
	if err := os.WriteFile(p, []byte(vulnModulesFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMatchDeps(t *testing.T) {
	dbPath := writeVulnDB(t, t.TempDir())
	db, err := LoadVulnDB(dbPath)
	if err != nil || db == nil {
		t.Fatalf("load: %v db=%v", err, db)
	}
	deps := map[string]string{
		"github.com/vuln/lib":  "v1.3.0", // < 1.4.0 (GO-0001) AND >= 1.2.0 -> only GO-0001
		"github.com/safe/lib":  "v2.1.0", // >= 2.0.0 -> clean
		"github.com/other/lib": "v0.1.0", // not in DB
	}
	f := matchDeps(deps, db)
	if len(f) != 1 {
		t.Fatalf("findings = %v, want 1 (only GO-2026-0001)", f)
	}
	if f[0].ID != "GO-2026-0001" || f[0].Module != "github.com/vuln/lib" || f[0].Fixed != "1.4.0" {
		t.Errorf("finding = %+v", f[0])
	}
}

func TestMatchDepsBelowBoth(t *testing.T) {
	dbPath := writeVulnDB(t, t.TempDir())
	db, _ := LoadVulnDB(dbPath)
	f := matchDeps(map[string]string{"github.com/vuln/lib": "v1.1.0"}, db) // < both fixed
	if len(f) != 2 {
		t.Fatalf("v1.1.0 should hit both advisories, got %v", f)
	}
}

func TestLoadVulnDBAbsent(t *testing.T) {
	if db, err := LoadVulnDB(""); db != nil || err != nil {
		t.Errorf("empty path -> nil,nil; got %v,%v", db, err)
	}
	if db, err := LoadVulnDB("/no/such/file.json"); db != nil || err != nil {
		t.Errorf("absent file -> nil,nil (no-op); got %v,%v", db, err)
	}
}

func TestParseModDeps(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod": "module x\n\nrequire (\n\tgithub.com/vuln/lib v1.3.0\n\tgithub.com/other v0.5.0 // indirect\n)\n\nreplace github.com/vuln/lib => github.com/vuln/lib v1.4.1\n",
	})
	deps := parseModDeps(root)
	if deps["github.com/vuln/lib"] != "v1.4.1" { // replace version override applied
		t.Errorf("replace not applied: %v", deps["github.com/vuln/lib"])
	}
	if deps["github.com/other"] != "v0.5.0" {
		t.Errorf("indirect require missed: %v", deps)
	}
}

func TestVulnInReport(t *testing.T) {
	root := t.TempDir()
	dbDir := t.TempDir()
	dbPath := writeVulnDB(t, dbDir)
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[{"name":"s"}]}`,
		"s/.git/HEAD":             "x",
		"s/AGENTS.md":             "a",
		"s/go.mod":                "module s\nrequire github.com/vuln/lib v1.0.0\n",
		"s/t.go":                  "package s\n\nimport \"x/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"t\") }\n",
	})
	rep, err := Scan(context.Background(), root, ScanOptions{VulnDBPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Repos[0].VulnFindings) != 2 { // v1.0.0 < both fixed
		t.Fatalf("expected 2 vuln findings, got %+v", rep.Repos[0].VulnFindings)
	}
	md := RenderMarkdown(rep)
	if !contains(md, "Dependency vulnerabilities (advisory)") {
		t.Error("markdown missing vuln section")
	}
}
