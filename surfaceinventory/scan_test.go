package surfaceinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRepo lays a fixture repo under root with the given files.
func writeRepo(t *testing.T, root, repo string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const mcpkitStyle = `package a

import (
	"context"

	"example.com/x/handler"
)

func tools() {
	_ = handler.TypedHandler[In, Out](
		"alpha_scan",
		"Scan alpha things.",
		func(ctx context.Context, in In) (Out, error) { return Out{}, nil },
	)
}
`

const mcpGoStyle = `package b

import "example.com/x/mcp"

func tools() {
	_ = mcp.NewTool("beta_sync", mcp.WithDescription("Sync beta things."))
	_ = mcp.NewResource("beta://state", "beta state")
	_ = mcp.NewPrompt("beta_prompt", mcp.WithPromptDescription("Prompt for beta."))
}
`

const officialStyle = `package c

import "example.com/x/mcp"

func register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "gamma_probe", Description: "Probe gamma."}, nil)
	_ = &mcp.Resource{URI: "gamma://config", Description: "Gamma config."}
	_ = &mcp.Prompt{Name: "gamma_prompt", Description: "Gamma prompt."}
}
`

const codexkitStyle = `package d

func defs() []ToolDef {
	return []ToolDef{
		{Name: "delta_check", Description: "Check delta."},
		{Name: "delta_report", Description: "Report delta."},
	}
}

type ToolDef struct {
	Name        string
	Description string
}
`

const cliHTTPStyle = `package e

import (
	"flag"
	"net/http"

	"example.com/x/cobra"
)

func surfaces(mux *http.ServeMux) {
	_ = cobra.Command{Use: "serve [flags]", Short: "Run the server."}
	_ = flag.NewFlagSet("migrate", flag.ContinueOnError)
	mux.HandleFunc("GET /api/health", nil)
	mux.Handle("/metrics", nil)
}
`

func TestScanWorkspaceExtractsAllPatterns(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{
		"a/a.go": mcpkitStyle,
		"b/b.go": mcpGoStyle,
		"c/c.go": officialStyle,
		"d/d.go": codexkitStyle,
		"e/e.go": cliHTTPStyle,
		// ignored: tests, vendored code, hidden dirs
		"a/a_test.go":      `package a; func x() { _ = "not scanned" }`,
		"vendor/v/v.go":    `package v`,
		".agents/agent.go": `package agent`,
	})

	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.ReposScanned != 1 {
		t.Fatalf("repos scanned = %d, want 1", rep.ReposScanned)
	}
	inv := rep.Repos[0]
	if len(inv.ParseErrors) != 0 {
		t.Fatalf("parse errors: %v", inv.ParseErrors)
	}

	wantCounts := map[string]int{
		KindMCPTool:     5, // alpha_scan, beta_sync, gamma_probe, delta_check, delta_report
		KindMCPResource: 2, // beta://state, gamma://config
		KindMCPPrompt:   2, // beta_prompt, gamma_prompt
		KindCLICommand:  2, // serve, migrate
		KindHTTPRoute:   2, // GET /api/health, /metrics
	}
	for kind, want := range wantCounts {
		if got := inv.Counts[kind]; got != want {
			t.Errorf("count[%s] = %d, want %d\nsurfaces: %+v", kind, got, want, inv.Surfaces)
		}
	}

	byName := map[string]Surface{}
	for _, s := range inv.Surfaces {
		byName[s.Name] = s
	}
	checks := []struct{ name, pattern, desc string }{
		{"alpha_scan", "mcpkit.TypedHandler", "Scan alpha things."},
		{"beta_sync", "mcp-go.NewTool", "Sync beta things."},
		{"gamma_probe", "official-sdk.Tool", "Probe gamma."},
		{"delta_check", "codexkit.ToolDef", "Check delta."},
		{"serve", "cobra.Command", "Run the server."},
		{"GET /api/health", "http.HandleFunc", ""},
	}
	for _, c := range checks {
		s, ok := byName[c.name]
		if !ok {
			t.Errorf("surface %q not extracted", c.name)
			continue
		}
		if s.Pattern != c.pattern {
			t.Errorf("%q pattern = %q, want %q", c.name, s.Pattern, c.pattern)
		}
		if s.Description != c.desc {
			t.Errorf("%q description = %q, want %q", c.name, s.Description, c.desc)
		}
		if s.File == "" || s.Line == 0 {
			t.Errorf("%q missing file:line (%q:%d)", c.name, s.File, s.Line)
		}
	}
}

func TestScanWorkspaceKindFilter(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{"b/b.go": mcpGoStyle, "e/e.go": cliHTTPStyle})

	rep, err := ScanWorkspace(root, []string{"fixture"}, []string{KindHTTPRoute})
	if err != nil {
		t.Fatal(err)
	}
	inv := rep.Repos[0]
	if len(inv.Counts) != 1 || inv.Counts[KindHTTPRoute] != 2 {
		t.Fatalf("kind filter leaked: %v", inv.Counts)
	}
}

func TestWorkspaceReposFromManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":1,"repos":[{"name":".github"},{"name":"alpha"},{"name":"beta"}]}`
	if err := os.WriteFile(filepath.Join(root, "workspace", "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	repos, err := WorkspaceRepos(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 || repos[0] != "alpha" || repos[1] != "beta" {
		t.Fatalf("repos = %v, want [alpha beta] (.github excluded)", repos)
	}
}

func TestScanWorkspaceSkipsMissingRepos(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "present", map[string]string{"p.go": `package p`})

	rep, err := ScanWorkspace(root, []string{"present", "absent"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.ReposScanned != 1 {
		t.Fatalf("repos scanned = %d, want 1 (absent skipped)", rep.ReposScanned)
	}
}

func TestParseErrorRecordedNotFatal(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{
		"bad.go":  `package broken func {{{`,
		"good.go": mcpGoStyle,
	})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	inv := rep.Repos[0]
	if len(inv.ParseErrors) != 1 {
		t.Fatalf("parse errors = %v, want exactly 1", inv.ParseErrors)
	}
	if inv.Counts[KindMCPTool] != 1 {
		t.Fatalf("good file not scanned past bad file: %v", inv.Counts)
	}
}

func TestRenderMarkdown(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{"b/b.go": mcpGoStyle})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(rep)
	for _, want := range []string{"# Fleet Surface Inventory", "| fixture |", "**total**"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestBareToolLiteralNotCounted(t *testing.T) {
	// Unqualified Tool{} literals (e.g. a local struct named Tool) must not
	// be counted as official-SDK registrations; only pkg-qualified ones are.
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{"x.go": `package x

type Tool struct{ Name string }

func f() { _ = Tool{Name: "not_a_tool"} }
`})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := rep.Repos[0].Counts[KindMCPTool]; n != 0 {
		t.Fatalf("bare Tool literal counted: %d", n)
	}
}

const fastMCPStyle = `import x

@mcp.tool()
async def audio_sinks() -> str:
    """List audio output sinks."""
    return "ok"

@mcp.tool(name="named_tool", description="Explicit kwargs.")
def whatever():
    pass

@mcp.tool()
def no_docstring():
    return 1
@mcp.resource("pi://status")
def status_resource():
    """Live status resource."""
    return {}

@mcp.prompt()
def diagnose():
    """Diagnose prompt."""
    return ""
`

func TestScanPythonFastMCP(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{
		"src/pkg/audio.py": fastMCPStyle,
		"src/test_skip.py": "@mcp.tool()\ndef skipped(): pass\n",
		"venv/lib/bad.py":  "@mcp.tool()\ndef venv_noise(): pass\n",
	})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	inv := rep.Repos[0]
	if inv.Counts[KindMCPTool] != 3 || inv.Counts[KindMCPResource] != 1 || inv.Counts[KindMCPPrompt] != 1 {
		t.Fatalf("counts = %v, want 3 tools / 1 resource / 1 prompt\n%+v", inv.Counts, inv.Surfaces)
	}
	byName := map[string]Surface{}
	for _, s := range inv.Surfaces {
		byName[s.Name] = s
	}
	if s := byName["audio_sinks"]; s.Description != "List audio output sinks." || s.Pattern != "fastmcp.decorator" {
		t.Errorf("audio_sinks = %+v", s)
	}
	if s := byName["named_tool"]; s.Description != "Explicit kwargs." {
		t.Errorf("named_tool = %+v", s)
	}
	if s, ok := byName["no_docstring"]; !ok || s.Description != "" {
		t.Errorf("no_docstring = %+v ok=%v", s, ok)
	}
	if s := byName["pi://status"]; s.Kind != KindMCPResource || s.Description != "Live status resource." {
		t.Errorf("resource = %+v", s)
	}
	if s := byName["diagnose"]; s.Kind != KindMCPPrompt {
		t.Errorf("prompt = %+v", s)
	}
	if _, ok := byName["skipped"]; ok {
		t.Error("test_*.py file was scanned")
	}
	if _, ok := byName["venv_noise"]; ok {
		t.Error("venv dir was scanned")
	}
}

func TestNestedRepoSkipped(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{
		"own.go":               "package x\n\nimport \"m/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"own_tool\") }\n",
		"oss/sub/.git":         "gitdir: ../../.git/modules/sub",
		"oss/sub/vendored.go":  "package s\n\nimport \"m/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"vendored_tool\") }\n",
		"embedded/.git/HEAD":   "ref: refs/heads/main",
		"embedded/embedded.go": "package e\n\nimport \"m/mcp\"\n\nfunc f() { _ = mcp.NewTool(\"embedded_tool\") }\n",
	})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	inv := rep.Repos[0]
	if inv.Counts[KindMCPTool] != 1 {
		t.Fatalf("counts = %v, want only own_tool\n%+v", inv.Counts, inv.Surfaces)
	}
	if inv.Surfaces[0].Name != "own_tool" {
		t.Errorf("surface = %+v", inv.Surfaces[0])
	}
}

func TestNonLiteralDescriptionCredited(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "fixture", map[string]string{
		"a.go": `package a

import "x/mcp"
import "x/handler"

func f() {
	// literal description
	_ = handler.TypedHandler[In, Out]("t_literal", "Real desc.", nil)
	// non-literal 2nd arg — description present but scanner can't read it
	desc := "built at runtime"
	_ = handler.TypedHandler[In, Out]("t_local", desc, nil)
	// mcp-go with non-literal WithDescription arg
	_ = mcp.NewTool("t_concat", mcp.WithDescription("a"+"b"))
	// mcp-go with NO description option at all
	_ = mcp.NewTool("t_none")
	// composite with non-literal Description field
	_ = &mcp.Tool{Name: "t_field", Description: desc}
}
`,
	})
	rep, err := ScanWorkspace(root, []string{"fixture"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Surface{}
	for _, s := range rep.Repos[0].Surfaces {
		byName[s.Name] = s
	}
	// present-but-non-literal → HasDescription true, Description empty
	for _, n := range []string{"t_literal", "t_local", "t_concat", "t_field"} {
		if !byName[n].HasDescription {
			t.Errorf("%s: HasDescription should be true (%+v)", n, byName[n])
		}
	}
	if byName["t_literal"].Description != "Real desc." {
		t.Errorf("literal desc lost: %+v", byName["t_literal"])
	}
	if byName["t_local"].Description != "" {
		t.Errorf("t_local should have empty literal desc: %+v", byName["t_local"])
	}
	// genuinely absent → HasDescription false
	if byName["t_none"].HasDescription {
		t.Errorf("t_none has no description arg, HasDescription must be false: %+v", byName["t_none"])
	}
}
