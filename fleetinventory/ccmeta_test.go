package fleetinventory

import (
	"context"
	"testing"
)

func TestDetectCCMeta(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a.go": `package a

func reg() {
	td := handler.TypedHandler("hot", "d", nil)
	td.AlwaysLoad = true
	td2 := codexkit.ToolDef{Name: "big", MaxResultChars: 100000, AlwaysLoad: true}
	_ = td2
}

type ToolDefinition struct {
	AlwaysLoad bool // definition — must NOT count
}

func read(td ToolDefinition) { if td.AlwaysLoad { _ = td } } // read-site — must NOT count
`,
		"b.go": `package b

var m = map[string]any{"anthropic/alwaysLoad": true}
`,
	})
	p := detectCCMeta(root, []string{"a.go", "b.go"})
	if p.AlwaysLoad != 2 { // td.AlwaysLoad = true  +  AlwaysLoad: true (composite)
		t.Errorf("AlwaysLoad = %d, want 2 (def + read-site excluded)", p.AlwaysLoad)
	}
	if p.MaxResultChars != 1 {
		t.Errorf("MaxResultChars = %d, want 1", p.MaxResultChars)
	}
	if p.LiteralMetaKeys != 1 {
		t.Errorf("LiteralMetaKeys = %d, want 1", p.LiteralMetaKeys)
	}
	if !p.Any() {
		t.Error("Any() should be true")
	}
}

func TestCCMetaCleanRepo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "package a\n\ntype T struct{ AlwaysLoad bool }\n"})
	p := detectCCMeta(root, []string{"a.go"})
	if p.Any() {
		t.Errorf("struct definition only should not count as adoption: %+v", p)
	}
}

func TestCCMetaOveruseWarning(t *testing.T) {
	root := t.TempDir()
	body := "package a\n\nfunc f() {\n"
	for i := 0; i < 10; i++ {
		body += "\t_ = ToolDef{AlwaysLoad: true}\n"
	}
	body += "}\n"
	writeFiles(t, root, map[string]string{"a.go": body})
	p := detectCCMeta(root, []string{"a.go"})
	if p.AlwaysLoad != 10 || p.Warning == "" {
		t.Errorf("expected overuse warning at 10 alwaysLoad, got count=%d warn=%q", p.AlwaysLoad, p.Warning)
	}
}

func TestCCMetaInReport(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[{"name":"srv"}]}`,
		"srv/.git/HEAD":           "x",
		"srv/AGENTS.md":           "a",
		"srv/go.mod":              "module srv\nrequire github.com/mark3labs/mcp-go v0.57.0\n",
		"srv/t.go":                "package srv\n\nimport \"x/mcp\"\n\nfunc f() { td := mcp.NewTool(\"t\"); _ = td }\n",
	})
	rep, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(rep)
	if !contains(md, "Claude Code _meta extensions") {
		t.Error("markdown missing CC _meta section")
	}
}
