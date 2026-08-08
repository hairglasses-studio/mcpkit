// Package surfaceinventory provides fleet-wide static enumeration of codebase
// surfaces: MCP tool/resource/prompt registrations, CLI subcommands, and HTTP
// routes, extracted from Go source with parse-only AST inspection (no type
// checking, no external dependencies).
//
// It recognizes every MCP registration idiom in use across the fleet:
// mcpkit's handler.TypedHandler, mark3labs/mcp-go's mcp.NewTool, the official
// go-sdk's mcp.AddTool / &mcp.Tool{...} literal, and codexkit's ToolDef{}
// slice literals — the gap left by per-server tool catalogs, which can only
// introspect their own registry.
package surfaceinventory

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Surface kinds.
const (
	KindMCPTool     = "mcp_tool"
	KindMCPResource = "mcp_resource"
	KindMCPPrompt   = "mcp_prompt"
	KindCLICommand  = "cli_command"
	KindHTTPRoute   = "http_route"
)

// AllKinds lists every surface kind the scanner emits.
var AllKinds = []string{KindMCPTool, KindMCPResource, KindMCPPrompt, KindCLICommand, KindHTTPRoute}

// Surface is one extracted registration site.
type Surface struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// HasDescription is true when a description slot is syntactically present
	// at the registration site — even when Description is empty because the
	// value is a non-literal expression (a `desc := ...` local, a `+`-
	// concatenated string, a call). Lets coverage scoring credit real-but-
	// non-literal descriptions instead of counting a scanner limitation as an
	// undocumented tool.
	HasDescription bool   `json:"has_description,omitempty"`
	Pattern        string `json:"pattern"`
	File           string `json:"file"`
	Line           int    `json:"line"`
	// Params carries per-parameter metadata when statically extractable — today
	// only the mcp-go WithString/WithNumber/… inline option pattern. Struct-
	// based inputs (mcpkit TypedHandler, official-SDK generics) need cross-file
	// type resolution and are not populated (nil), which is distinct from a
	// genuinely param-less tool (empty non-nil is not produced).
	Params []ToolParam `json:"params,omitempty"`
}

// ToolParam is one statically-extracted input parameter of an MCP tool.
type ToolParam struct {
	Name           string `json:"name"`
	Type           string `json:"type,omitempty"` // String, Number, Boolean, Array, Object
	HasDescription bool   `json:"has_description,omitempty"`
	Required       bool   `json:"required,omitempty"`
	HasEnum        bool   `json:"has_enum,omitempty"`
}

// RepoInventory is the scan result for one repo.
type RepoInventory struct {
	Repo        string         `json:"repo"`
	Counts      map[string]int `json:"counts"`
	Surfaces    []Surface      `json:"surfaces,omitempty"`
	FilesParsed int            `json:"files_parsed"`
	ParseErrors []string       `json:"parse_errors,omitempty"`
}

// Report aggregates per-repo inventories across a workspace.
type Report struct {
	Root         string          `json:"root"`
	Repos        []RepoInventory `json:"repos"`
	Totals       map[string]int  `json:"totals"`
	ReposScanned int             `json:"repos_scanned"`
}

// skipDirs are directory basenames never descended into.
var skipDirs = map[string]bool{
	"vendor":        true,
	"node_modules":  true,
	"testdata":      true,
	"worktrees":     true,
	"venv":          true,
	"site-packages": true,
	"__pycache__":   true,
	"examples":      true,
	"_reference":    true,
}

type manifestFile struct {
	Repos []struct {
		Name string `json:"name"`
	} `json:"repos"`
}

// WorkspaceRepos resolves the repo list for a workspace root: the explicit
// list if non-empty, else workspace/manifest.json, else immediate
// subdirectories containing a .git entry.
func WorkspaceRepos(root string, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "workspace", "manifest.json"))
	if err == nil {
		var m manifestFile
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parse workspace manifest: %w", err)
		}
		names := make([]string, 0, len(m.Repos))
		for _, r := range m.Repos {
			if r.Name != "" && r.Name != ".github" {
				names = append(names, r.Name)
			}
		}
		if len(names) > 0 {
			return names, nil
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read workspace root: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), ".git")); err == nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ScanWorkspace scans the given repos (see WorkspaceRepos) under root.
// Missing repo directories are skipped silently so a stale manifest does not
// fail the whole sweep; kinds filters the emitted surface kinds (empty = all).
func ScanWorkspace(root string, repos []string, kinds []string) (Report, error) {
	names, err := WorkspaceRepos(root, repos)
	if err != nil {
		return Report{}, err
	}
	kindSet := kindFilter(kinds)
	report := Report{Root: root, Totals: map[string]int{}}
	for _, name := range names {
		dir := filepath.Join(root, name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		inv := ScanRepo(dir, name, kindSet)
		report.Repos = append(report.Repos, inv)
		report.ReposScanned++
		for k, v := range inv.Counts {
			report.Totals[k] += v
		}
	}
	sort.Slice(report.Repos, func(i, j int) bool { return report.Repos[i].Repo < report.Repos[j].Repo })
	return report, nil
}

// ScanFiles extracts surfaces from the given repo-relative files (callers
// that already hold a file listing avoid a second walk). Non-Go and _test.go
// entries are skipped; parse failures are recorded per-file, never fatal.
func ScanFiles(dir, name string, files []string, kindSet map[string]bool) RepoInventory {
	inv := RepoInventory{Repo: name, Counts: map[string]int{}}
	fset := token.NewFileSet()

	emit := func(surfaces []Surface) {
		inv.FilesParsed++
		for _, s := range surfaces {
			if kindSet != nil && !kindSet[s.Kind] {
				continue
			}
			inv.Surfaces = append(inv.Surfaces, s)
			inv.Counts[s.Kind]++
		}
	}

	// Group .go files by package directory so a same-package struct index can
	// resolve TypedHandler input types declared in a sibling file.
	goByDir := map[string][]string{}
	var pyFiles []string
	for _, rel := range files {
		switch {
		case strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go"):
			d := slashDir(rel)
			goByDir[d] = append(goByDir[d], rel)
		case isScannablePython(rel):
			pyFiles = append(pyFiles, rel)
		}
	}

	for _, group := range goByDir {
		si := newStructIndex()
		parsed := make(map[string]*ast.File, len(group))
		for _, rel := range group {
			path := filepath.Join(dir, filepath.FromSlash(rel))
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				inv.ParseErrors = append(inv.ParseErrors, fmt.Sprintf("%s: %v", rel, err))
				continue
			}
			parsed[rel] = file
			si.indexFile(file)
		}
		for _, rel := range group {
			if file := parsed[rel]; file != nil {
				emit(extractFile(fset, file, rel, si))
			}
		}
	}

	for _, rel := range pyFiles {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		surfaces, err := scanPythonFile(path, rel)
		if err != nil {
			inv.ParseErrors = append(inv.ParseErrors, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		emit(surfaces)
	}

	sortSurfaces(inv.Surfaces)
	return inv
}

// slashDir returns the directory of a forward-slash relative path ("." for a
// top-level file).
func slashDir(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return "."
}

// ScanRepo walks one repo directory and extracts surfaces from every non-test
// Go and Python source file. Parse failures are recorded per-file, never fatal.
func ScanRepo(dir, name string, kindSet map[string]bool) RepoInventory {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != dir && (strings.HasPrefix(base, ".") || skipDirs[base]) {
				return filepath.SkipDir
			}
			if path != dir && isNestedRepo(path) {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, relPath(dir, path))
		return nil
	})
	return ScanFiles(dir, name, files, kindSet)
}

// isNestedRepo reports whether dir is a nested git checkout (a submodule's
// gitlink file or an embedded .git dir). Nested repos are inventoried under
// their own fleet entry; counting them in the parent double-counts.
func isNestedRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func sortSurfaces(surfaces []Surface) {
	sort.Slice(surfaces, func(i, j int) bool {
		a, b := surfaces[i], surfaces[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.File+strconv.Itoa(a.Line) < b.File+strconv.Itoa(b.Line)
	})
}

func kindFilter(kinds []string) map[string]bool {
	if len(kinds) == 0 {
		return nil
	}
	set := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		set[strings.TrimSpace(k)] = true
	}
	return set
}

func relPath(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return rel
}

// extractFile pulls every recognized surface out of one parsed file. si is the
// same-package struct index used to resolve TypedHandler input parameters.
func extractFile(fset *token.FileSet, file *ast.File, relFile string, si *structIndex) []Surface {
	var out []Surface
	add := func(node ast.Node, kind, name, desc, pattern string, hasDesc bool, params []ToolParam) {
		if name == "" {
			return
		}
		out = append(out, Surface{
			Kind: kind, Name: name, Description: desc, HasDescription: hasDesc || desc != "",
			Pattern: pattern, File: relFile, Line: fset.Position(node.Pos()).Line, Params: params,
		})
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			extractCall(node, add, si)
		case *ast.CompositeLit:
			extractComposite(node, add)
		}
		return true
	})
	return out
}

type addFunc func(node ast.Node, kind, name, desc, pattern string, hasDesc bool, params []ToolParam)

func extractCall(call *ast.CallExpr, add addFunc, si *structIndex) {
	callee := calleeName(call.Fun)
	switch callee {
	case "TypedHandler": // mcpkit: handler.TypedHandler[In, Out]("name", "desc", fn)
		if len(call.Args) >= 2 {
			// The 2nd arg is the description slot whether or not it's a literal.
			// Params are resolved from the In type struct via the same-package index.
			add(call, KindMCPTool, stringLit(call.Args[0]), stringLit(call.Args[1]), "mcpkit.TypedHandler", true,
				resolveStructParams(call, si))
		}
	case "NewTool": // mcp-go: mcp.NewTool("name", mcp.WithDescription("desc"), mcp.WithString("p", ...), ...)
		if len(call.Args) >= 1 {
			add(call, KindMCPTool, stringLit(call.Args[0]), optionString(call.Args, "WithDescription"),
				"mcp-go.NewTool", optionPresent(call.Args, "WithDescription"), extractParams(call.Args))
		}
	case "NewResource", "NewResourceTemplate": // mcp-go + mcpkit compat: (uri, name, opts...)
		if len(call.Args) >= 1 {
			desc := ""
			if len(call.Args) >= 2 {
				desc = stringLit(call.Args[1])
			}
			add(call, KindMCPResource, stringLit(call.Args[0]), desc, callee, len(call.Args) >= 2, nil)
		}
	case "NewPrompt": // mcp-go: mcp.NewPrompt("name", opts...)
		if len(call.Args) >= 1 {
			add(call, KindMCPPrompt, stringLit(call.Args[0]), optionString(call.Args, "WithPromptDescription"),
				"mcp-go.NewPrompt", optionPresent(call.Args, "WithPromptDescription"), nil)
		}
	case "NewFlagSet": // stdlib flag: flag.NewFlagSet("name", ...)
		if len(call.Args) >= 1 {
			add(call, KindCLICommand, stringLit(call.Args[0]), "", "flag.NewFlagSet", false, nil)
		}
	case "Handle", "HandleFunc": // net/http muxes and gateway routers
		if len(call.Args) >= 1 {
			if route := stringLit(call.Args[0]); strings.Contains(route, "/") {
				add(call, KindHTTPRoute, route, "", "http."+callee, false, nil)
			}
		}
	}
}

// paramDefiners are the mcp-go option calls that declare a tool input
// parameter: mcp.WithString("name", opts...), WithNumber, WithBoolean, etc.
var paramDefiners = map[string]bool{
	"WithString": true, "WithNumber": true, "WithBoolean": true,
	"WithArray": true, "WithObject": true, "WithInteger": true,
}

// extractParams pulls per-parameter metadata from the mcp-go WithXxx option
// calls in a NewTool argument list. Each param's property options
// (mcp.Description / mcp.Required / mcp.Enum) are inspected for presence.
func extractParams(args []ast.Expr) []ToolParam {
	var params []ToolParam
	for _, a := range args {
		call, ok := a.(*ast.CallExpr)
		if !ok || !paramDefiners[calleeName(call.Fun)] || len(call.Args) < 1 {
			continue
		}
		pname := stringLit(call.Args[0])
		if pname == "" {
			continue
		}
		p := ToolParam{Name: pname, Type: strings.TrimPrefix(calleeName(call.Fun), "With")}
		for _, opt := range call.Args[1:] {
			oc, ok := opt.(*ast.CallExpr)
			if !ok {
				continue
			}
			switch calleeName(oc.Fun) {
			case "Description":
				p.HasDescription = true
			case "Required":
				p.Required = true
			case "Enum":
				p.HasEnum = true
			}
		}
		params = append(params, p)
	}
	return params
}

func extractComposite(lit *ast.CompositeLit, add addFunc) {
	// Slice literals like []ToolDef{{Name: ...}, ...} carry the type on the
	// array, with elided types on the elements — process each element under
	// the array's element type.
	if arr, ok := lit.Type.(*ast.ArrayType); ok {
		typeName, qualified := compositeTypeName(arr.Elt)
		for _, e := range lit.Elts {
			if inner, ok := e.(*ast.CompositeLit); ok && inner.Type == nil {
				emitComposite(inner, typeName, qualified, add)
			}
		}
		return
	}
	typeName, qualified := compositeTypeName(lit.Type)
	emitComposite(lit, typeName, qualified, add)
}

func emitComposite(lit *ast.CompositeLit, typeName string, qualified bool, add addFunc) {
	switch typeName {
	case "Tool": // official go-sdk: &mcp.Tool{Name: ..., Description: ...}
		if qualified {
			if name := fieldString(lit, "Name"); name != "" {
				add(lit, KindMCPTool, name, fieldString(lit, "Description"), "official-sdk.Tool", fieldPresent(lit, "Description"), nil)
			}
		}
	case "ToolDef": // codexkit: codexkit.ToolDef{Name: ..., Description: ...}
		if name := fieldString(lit, "Name"); name != "" {
			add(lit, KindMCPTool, name, fieldString(lit, "Description"), "codexkit.ToolDef", fieldPresent(lit, "Description"), nil)
		}
	case "Resource", "ResourceTemplate": // official go-sdk resource literals
		if qualified {
			if uri := firstNonEmpty(fieldString(lit, "URI"), fieldString(lit, "URITemplate")); uri != "" {
				add(lit, KindMCPResource, uri, fieldString(lit, "Description"), "official-sdk."+typeName, fieldPresent(lit, "Description"), nil)
			}
		}
	case "Prompt", "PromptDefinition": // official go-sdk / mcpkit prompt literals
		if name := fieldString(lit, "Name"); name != "" {
			add(lit, KindMCPPrompt, name, fieldString(lit, "Description"), typeName, fieldPresent(lit, "Description"), nil)
		}
	case "Command": // cobra.Command{Use: "serve [flags]"}
		if use := fieldString(lit, "Use"); use != "" {
			name := strings.Fields(use)[0]
			add(lit, KindCLICommand, name, fieldString(lit, "Short"), "cobra.Command", fieldPresent(lit, "Short"), nil)
		}
	}
}

// calleeName unwraps selector and generic-instantiation wrappers around a
// call target and returns the bare function name.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.IndexExpr: // one type parameter
		return calleeName(f.X)
	case *ast.IndexListExpr: // multiple type parameters (TypedHandler[In, Out])
		return calleeName(f.X)
	}
	return ""
}

// compositeTypeName returns the type's bare name and whether it was
// package-qualified (pkg.Tool vs bare Tool).
func compositeTypeName(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, false
	case *ast.SelectorExpr:
		return t.Sel.Name, true
	}
	return "", false
}

// stringLit returns the value of a string literal expression, else "".
func stringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return val
}

// optionPresent reports whether an option call like mcp.WithDescription(...)
// appears among the args at all, regardless of whether its argument is a
// string literal the scanner can read.
func optionPresent(args []ast.Expr, option string) bool {
	for _, arg := range args {
		if call, ok := arg.(*ast.CallExpr); ok && calleeName(call.Fun) == option {
			return true
		}
	}
	return false
}

// fieldPresent reports whether a named field key appears in a composite
// literal, regardless of whether its value is a string literal.
func fieldPresent(lit *ast.CompositeLit, field string) bool {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
				return true
			}
		}
	}
	return false
}

// optionString finds an option-call argument like mcp.WithDescription("...")
// among call args and returns its first string literal.
func optionString(args []ast.Expr, option string) string {
	for _, arg := range args {
		call, ok := arg.(*ast.CallExpr)
		if !ok || calleeName(call.Fun) != option || len(call.Args) == 0 {
			continue
		}
		if s := stringLit(call.Args[0]); s != "" {
			return s
		}
	}
	return ""
}

// fieldString returns the string literal assigned to a named field in a
// composite literal, else "".
func fieldString(lit *ast.CompositeLit, field string) string {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		return stringLit(kv.Value)
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
