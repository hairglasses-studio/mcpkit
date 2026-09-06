package fleetinventory

import (
	"context"
	"testing"
)

func TestDetectMCPRuntime(t *testing.T) {
	cases := []struct {
		name    string
		gomod   string
		wantEra string
	}{
		{"modern", "module x\ngo 1.26\nrequire github.com/modelcontextprotocol/go-sdk v1.7.0\n", EraModernCapable},
		{"legacy-mark3", "module x\nrequire github.com/mark3labs/mcp-go v0.57.0\n", EraLegacyOnly},
		// mark3labs/mcp-go v1.0.0 set LATEST_PROTOCOL_VERSION to 2026-07-28,
		// so a v1+ pin is modern-capable. Classifying it legacy-only (the
		// pre-2026-09 behaviour) mislabels every repo on the new major.
		{"modern-mark3", "module x\nrequire github.com/mark3labs/mcp-go v1.0.0\n", EraModernCapable},
		{"dual-modern-mark3-old-official", "module x\nrequire (\n\tgithub.com/mark3labs/mcp-go v1.0.0\n\tgithub.com/modelcontextprotocol/go-sdk v1.4.1\n)\n", EraDual},
		{"legacy-old-official", "module x\nrequire github.com/modelcontextprotocol/go-sdk v1.4.1\n", EraLegacyOnly},
		{"dual", "module x\nrequire (\n\tgithub.com/mark3labs/mcp-go v0.54.0\n\tgithub.com/modelcontextprotocol/go-sdk v1.7.0\n)\n", EraDual},
		{"via-mcpkit", "module x\nrequire github.com/hairglasses-studio/mcpkit v0.8.0\n", EraViaMcpkit},
		{"none", "module x\nrequire github.com/spf13/cobra v1.8.0\n", EraNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"go.mod": c.gomod})
			rt := detectMCPRuntime(root)
			if rt.SpecEra != c.wantEra {
				t.Errorf("era = %q, want %q (sdks=%v)", rt.SpecEra, c.wantEra, rt.SDKs)
			}
		})
	}
}

func TestSemver(t *testing.T) {
	if !semverAtLeast("v1.7.0", 1, 7) {
		t.Error("v1.7.0 should be >= 1.7")
	}
	if semverAtLeast("v1.6.9", 1, 7) {
		t.Error("v1.6.9 should be < 1.7")
	}
	if !semverAtLeast("v2.0.0", 1, 7) {
		t.Error("v2.0.0 should be >= 1.7")
	}
	if !semverGreater("v0.57.0", "v0.54.0") {
		t.Error("0.57 > 0.54")
	}
}

func TestMCPRuntimeInReport(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"workspace/manifest.json": `{"version":1,"repos":[{"name":"srv"}]}`,
		"srv/.git/HEAD":           "x",
		"srv/AGENTS.md":           "a",
		"srv/go.mod":              "module srv\nrequire github.com/mark3labs/mcp-go v0.57.0\n",
		"srv/main.go":             "package main\n\nimport \"x/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"t\", mcp.WithDescription(\"d\")) }\n",
	})
	rep, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Repos[0].MCPRuntime.SpecEra != EraLegacyOnly {
		t.Errorf("era = %q, want legacy-only", rep.Repos[0].MCPRuntime.SpecEra)
	}
}
