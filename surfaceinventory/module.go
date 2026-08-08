package surfaceinventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// defaultMaxSurfacesPerRepo bounds per-repo surface listings so a full fleet
// sweep (30+ repos, some with 600+ tools) stays under MCP response limits.
const defaultMaxSurfacesPerRepo = 200

type scanInput struct {
	Root               string   `json:"root" jsonschema:"required,description=Workspace root directory containing the repos to scan"`
	Repos              []string `json:"repos,omitempty" jsonschema:"description=Repo names to scan. Omit to use workspace/manifest.json or .git subdirectory discovery."`
	Kinds              []string `json:"kinds,omitempty" jsonschema:"description=Surface kinds to include: mcp_tool mcp_resource mcp_prompt cli_command http_route. Omit for all."`
	IncludeSurfaces    bool     `json:"include_surfaces,omitempty" jsonschema:"description=Include per-surface detail rows (name/file/line) instead of counts only"`
	MaxSurfacesPerRepo int      `json:"max_surfaces_per_repo,omitempty" jsonschema:"description=Cap on detail rows per repo when include_surfaces is set (default 200)"`
}

type scanOutput struct {
	Report    Report `json:"report"`
	Truncated bool   `json:"truncated,omitempty"`
}

type reportInput struct {
	Root  string   `json:"root" jsonschema:"required,description=Workspace root directory containing the repos to scan"`
	Repos []string `json:"repos,omitempty" jsonschema:"description=Repo names to scan. Omit for full workspace."`
}

type reportOutput struct {
	Markdown string `json:"markdown"`
}

type module struct{}

// Module returns the surface-inventory ToolModule. Mount it into any server:
//
//	reg.RegisterModule(surfaceinventory.Module())
func Module() registry.ToolModule { return &module{} }

func (m *module) Name() string { return "surface_inventory" }
func (m *module) Description() string {
	return "Fleet-wide static inventory of MCP tools/resources/prompts, CLI commands, and HTTP routes across workspace repos"
}

func (m *module) Tools() []registry.ToolDefinition {
	scan := handler.TypedHandler[scanInput, scanOutput](
		"surface_inventory_scan",
		"Statically enumerate codebase surfaces (MCP tool/resource/prompt registrations across all fleet SDK idioms, CLI subcommands, HTTP routes) for every repo in a workspace. Returns per-repo counts; set include_surfaces for name/file/line detail.",
		func(_ context.Context, in scanInput) (scanOutput, error) {
			report, err := ScanWorkspace(in.Root, in.Repos, in.Kinds)
			if err != nil {
				return scanOutput{}, err
			}
			out := scanOutput{Report: report}
			maxPer := in.MaxSurfacesPerRepo
			if maxPer <= 0 {
				maxPer = defaultMaxSurfacesPerRepo
			}
			for i := range out.Report.Repos {
				if !in.IncludeSurfaces {
					out.Report.Repos[i].Surfaces = nil
					continue
				}
				if len(out.Report.Repos[i].Surfaces) > maxPer {
					out.Report.Repos[i].Surfaces = out.Report.Repos[i].Surfaces[:maxPer]
					out.Truncated = true
				}
			}
			return out, nil
		},
	)
	scan.Category = "audit"
	scan.SearchTerms = []string{"surface inventory", "fleet audit", "tool enumeration", "mcp registrations", "cli commands", "http routes"}

	report := handler.TypedHandler[reportInput, reportOutput](
		"surface_inventory_report",
		"Render the fleet surface inventory as a Markdown counts table (repos x surface kinds) with totals.",
		func(_ context.Context, in reportInput) (reportOutput, error) {
			rep, err := ScanWorkspace(in.Root, in.Repos, nil)
			if err != nil {
				return reportOutput{}, err
			}
			return reportOutput{Markdown: RenderMarkdown(rep)}, nil
		},
	)
	report.Category = "audit"
	report.SearchTerms = []string{"surface report", "inventory markdown", "fleet summary"}

	return []registry.ToolDefinition{scan, report}
}

// RenderMarkdown renders a Report as a repos-by-kinds counts table.
func RenderMarkdown(rep Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Fleet Surface Inventory\n\nRoot: `%s` — %d repos scanned\n\n", rep.Root, rep.ReposScanned)
	b.WriteString("| Repo |")
	for _, k := range AllKinds {
		fmt.Fprintf(&b, " %s |", k)
	}
	b.WriteString(" total |\n|---|")
	b.WriteString(strings.Repeat("---|", len(AllKinds)+1))
	b.WriteString("\n")

	repos := append([]RepoInventory(nil), rep.Repos...)
	sort.Slice(repos, func(i, j int) bool { return total(repos[i].Counts) > total(repos[j].Counts) })
	for _, r := range repos {
		fmt.Fprintf(&b, "| %s |", r.Repo)
		for _, k := range AllKinds {
			fmt.Fprintf(&b, " %d |", r.Counts[k])
		}
		fmt.Fprintf(&b, " %d |\n", total(r.Counts))
	}
	b.WriteString("| **total** |")
	for _, k := range AllKinds {
		fmt.Fprintf(&b, " **%d** |", rep.Totals[k])
	}
	fmt.Fprintf(&b, " **%d** |\n", total(rep.Totals))
	return b.String()
}

func total(counts map[string]int) int {
	n := 0
	for _, v := range counts {
		n += v
	}
	return n
}
