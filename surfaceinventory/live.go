package surfaceinventory

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// LiveDiffCounts summarizes the size of each partition in a LiveDiff.
type LiveDiffCounts struct {
	Live       int `json:"live"`
	Static     int `json:"static"`
	LiveOnly   int `json:"live_only"`
	StaticOnly int `json:"static_only"`
	Both       int `json:"both"`
}

// LiveDiff is the result of comparing the surface names a live MCP server
// actually reports at connect time against a static AST scan of the same
// repo's source. Both name sets are deduplicated before partitioning.
type LiveDiff struct {
	LiveOnly   []string       `json:"live_only"`
	StaticOnly []string       `json:"static_only"`
	Both       []string       `json:"both"`
	Counts     LiveDiffCounts `json:"counts"`
}

// diffNames partitions two name lists into live-only, static-only, and
// present-in-both sets, each sorted for deterministic output. This is a
// pure function with no I/O — the seam this package's tests exercise
// directly, since spinning up a real client/server pair over stdio belongs
// in an integration test (see live_official_test.go).
func diffNames(live, static []string) LiveDiff {
	liveSet := make(map[string]bool, len(live))
	for _, n := range live {
		liveSet[n] = true
	}
	staticSet := make(map[string]bool, len(static))
	for _, n := range static {
		staticSet[n] = true
	}

	var liveOnly, staticOnly, both []string
	for n := range liveSet {
		if staticSet[n] {
			both = append(both, n)
		} else {
			liveOnly = append(liveOnly, n)
		}
	}
	for n := range staticSet {
		if !liveSet[n] {
			staticOnly = append(staticOnly, n)
		}
	}
	sort.Strings(liveOnly)
	sort.Strings(staticOnly)
	sort.Strings(both)

	return LiveDiff{
		LiveOnly:   liveOnly,
		StaticOnly: staticOnly,
		Both:       both,
		Counts: LiveDiffCounts{
			Live:       len(liveSet),
			Static:     len(staticSet),
			LiveOnly:   len(liveOnly),
			StaticOnly: len(staticOnly),
			Both:       len(both),
		},
	}
}

// defaultLiveTimeout bounds how long surface_inventory_live waits for the
// spawned server to answer the list requests before giving up.
const defaultLiveTimeout = 20 * time.Second

// liveInput is the input for the surface_inventory_live tool.
type liveInput struct {
	Command        []string `json:"command" jsonschema:"required,description=Argv to launch the MCP server binary over stdio: argv[0] is the executable path, remaining elements are its arguments"`
	StaticRoot     string   `json:"static_root" jsonschema:"required,description=Workspace root (containing workspace/manifest.json or repo subdirectories) or a single repo directory to statically scan for comparison"`
	Repos          []string `json:"repos,omitempty" jsonschema:"description=Repo names under static_root to scan. Omit to auto-discover via manifest.json/.git subdirs, or if static_root is itself a single repo"`
	Kind           string   `json:"kind,omitempty" jsonschema:"description=Surface kind to compare: mcp_tool (default), mcp_resource, or mcp_prompt,enum=mcp_tool,enum=mcp_resource,enum=mcp_prompt"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"description=Max seconds to wait for the live server to respond (default 20)"`
}

// liveOutput is the output of the surface_inventory_live tool.
type liveOutput struct {
	LiveDiff
	Kind string `json:"kind"`
}

// connectAndListNames connects to the MCP server launched by argv over
// stdio and returns every live-registered name of the given kind
// (tool/prompt Name, or resource URI), auto-paginating through the full
// result set. Implemented per build tag: live_official.go (official_sdk,
// via go-sdk's mcp.CommandTransport + ClientSession's iterator methods) or
// live_default.go (default tag, a stub that reports the build-tag
// requirement). Declared here only in this doc comment — see those files
// for the actual (mutually exclusive) function definition.

// staticNamesForKind runs (or reuses) a static AST scan under root and
// returns every surface name of the given kind. When repos resolves to
// nothing (no manifest.json, no .git subdirectories — i.e. root itself is a
// single repo directory) it scans root directly under its base name.
func staticNamesForKind(root string, repos []string, kind string) ([]string, error) {
	var surfaces []Surface

	resolved, err := WorkspaceRepos(root, repos)
	if err == nil && len(resolved) > 0 {
		report, err := ScanWorkspace(root, resolved, []string{kind})
		if err != nil {
			return nil, fmt.Errorf("static scan: %w", err)
		}
		for _, r := range report.Repos {
			surfaces = append(surfaces, r.Surfaces...)
		}
	} else {
		inv := ScanRepo(root, filepath.Base(root), kindFilter([]string{kind}))
		surfaces = inv.Surfaces
	}

	names := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		names = append(names, s.Name)
	}
	return names, nil
}

// liveToolDef builds the surface_inventory_live tool definition. Split out
// of Tools() so live_official_test.go / live_default_test.go can construct
// it without depending on module registration order.
func liveToolDef() registry.ToolDefinition {
	td := handler.TypedHandler[liveInput, liveOutput](
		"surface_inventory_live",
		"Connect to a live MCP server over stdio and diff its actually-registered tool/resource/prompt names against a static AST scan of the same repo, surfacing registrations the static scanner misses (and vice versa).",
		func(ctx context.Context, in liveInput) (liveOutput, error) {
			if len(in.Command) == 0 {
				return liveOutput{}, fmt.Errorf("command is required (argv to launch the MCP server)")
			}
			if in.StaticRoot == "" {
				return liveOutput{}, fmt.Errorf("static_root is required")
			}

			kind := in.Kind
			if kind == "" {
				kind = KindMCPTool
			}
			switch kind {
			case KindMCPTool, KindMCPResource, KindMCPPrompt:
			default:
				return liveOutput{}, fmt.Errorf("kind must be one of %s, %s, %s (got %q)", KindMCPTool, KindMCPResource, KindMCPPrompt, kind)
			}

			timeout := time.Duration(in.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = defaultLiveTimeout
			}
			liveCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			liveNames, err := connectAndListNames(liveCtx, in.Command, kind)
			if err != nil {
				return liveOutput{}, err
			}

			staticNames, err := staticNamesForKind(in.StaticRoot, in.Repos, kind)
			if err != nil {
				return liveOutput{}, err
			}

			return liveOutput{LiveDiff: diffNames(liveNames, staticNames), Kind: kind}, nil
		},
	)
	td.Category = "audit"
	td.SearchTerms = []string{"live inventory", "declared vs scanned", "mcp client diff", "surface drift", "connect and enumerate"}
	td.Complexity = registry.ComplexityModerate
	return td
}
