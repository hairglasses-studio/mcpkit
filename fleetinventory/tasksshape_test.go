package fleetinventory

import (
	"context"
	"testing"
)

func TestDetectTasksShape(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"clean.go":     "package x\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(\"hi\") }\n",
		"imp.go":       "package x\n\nimport tasks \"github.com/hairglasses-studio/mcpkit/registry/tasks\"\n\nvar _ = tasks.Manager{}\n",
		"mcptype.go":   "package x\n\nfunc f(s mcp.TaskStatus) mcp.TaskSupport { return 0 }\n",
		"gtasks.go":    "package x\n\nfunc reg() { register(\"tasks/list\", nil) }\n",            // Google Tasks — must NOT flag
		"owntype.go":   "package x\n\ntype TaskStatus int\n\nfunc f() TaskStatus { return 0 }\n", // private type — must NOT flag
		"skip_test.go": "package x\n\nimport _ \"github.com/hairglasses-studio/mcpkit/registry/tasks\"\n",
	})
	files := []string{"clean.go", "imp.go", "mcptype.go", "gtasks.go", "owntype.go", "skip_test.go"}
	used, evidence := detectTasksShape(root, files)
	if !used {
		t.Fatal("expected tasks-shape usage detected")
	}
	if len(evidence) != 2 { // imp + mcptype only; gtasks/owntype/clean/_test excluded
		t.Errorf("evidence = %v, want 2 (import + mcp.Task type; generic tasks/list and private TaskStatus excluded)", evidence)
	}
}

func TestDetectTasksShapeClean(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "package x\n\nimport \"x/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"t\") }\n"})
	if used, _ := detectTasksShape(root, []string{"a.go"}); used {
		t.Error("clean repo should not flag tasks-shape")
	}
}

func TestTasksWarningSeverity(t *testing.T) {
	if w := tasksWarning(MCPRuntime{TasksLegacyShape: true, SpecEra: EraDual}); w == "" || !contains(w, "NON-CONFORMANT") {
		t.Errorf("modern/dual should be non-conformant: %q", w)
	}
	if w := tasksWarning(MCPRuntime{TasksLegacyShape: true, SpecEra: EraLegacyOnly}); w == "" || !contains(w, "migration hazard") {
		t.Errorf("legacy should be migration hazard: %q", w)
	}
	if w := tasksWarning(MCPRuntime{TasksLegacyShape: false, SpecEra: EraDual}); w != "" {
		t.Errorf("no tasks usage -> no warning, got %q", w)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestTasksShapeInReport(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[{"name":"srv"}]}`,
		"srv/.git/HEAD":           "x",
		"srv/AGENTS.md":           "a",
		"srv/go.mod":              "module srv\nrequire github.com/modelcontextprotocol/go-sdk v1.7.0\n",
		"srv/tasks.go":            "package srv\n\nimport tasks \"github.com/hairglasses-studio/mcpkit/registry/tasks\"\n\nvar _ = tasks.Manager{}\n",
	})
	rep, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rt := rep.Repos[0].MCPRuntime
	if !rt.TasksLegacyShape {
		t.Fatal("tasks-shape not detected in report")
	}
	md := RenderMarkdown(rep)
	if !contains(md, "Tasks-shape warnings") || !contains(md, "NON-CONFORMANT") {
		t.Error("markdown missing tasks-shape warning (modern-capable server)")
	}
}
