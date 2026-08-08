package fleetinventory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

// RepoReport is the full inventory for one repo.
type RepoReport struct {
	Repo          string                     `json:"repo"`
	Dir           string                     `json:"-"`
	Files         int                        `json:"files"`
	Truncated     bool                       `json:"truncated,omitempty"`
	WalkErrors    []string                   `json:"walk_errors,omitempty"`
	Parity        ParityMetrics              `json:"parity"`
	Surfaces      map[string]int             `json:"surface_counts"`
	SurfaceDetail []surfaceinventory.Surface `json:"surface_detail,omitempty"`
	MCPRuntime    MCPRuntime                 `json:"mcp_runtime"`
	CCMeta        CCMetaProfile              `json:"cc_meta"`
	VulnFindings  []VulnFinding              `json:"vuln_findings,omitempty"`
	ScanMillis    int64                      `json:"scan_millis"`
}

// Drift is the workspace-consistency section: disk vs manifest vs catalog.
type Drift struct {
	OnDiskOnly   []string `json:"on_disk_only,omitempty"`
	ManifestOnly []string `json:"manifest_only,omitempty"`
	CatalogOnly  []string `json:"catalog_only,omitempty"`
	CatalogPath  string   `json:"catalog_path,omitempty"`
	ManifestPath string   `json:"manifest_path,omitempty"`
}

// PlatformReport is the whole-fleet inventory.
type PlatformReport struct {
	GeneratedOn    string         `json:"generated_on"`
	Root           string         `json:"root"`
	Repos          []RepoReport   `json:"repos"`
	SurfaceTotals  map[string]int `json:"surface_totals"`
	ViolationCount int            `json:"violation_count"`
	Drift          Drift          `json:"drift"`
	Scoring        *ScoreReport   `json:"scoring,omitempty"`
	TotalMillis    int64          `json:"total_millis"`
}

// ScanOptions configure a platform scan.
type ScanOptions struct {
	Repos           []string
	IncludeSurfaces bool
	// Score computes the quality scoreboard. Scoring needs surface detail,
	// so detail is collected internally regardless of IncludeSurfaces (and
	// stripped from the output again unless IncludeSurfaces was set) —
	// otherwise most dimensions silently degrade to violation-only scoring.
	Score bool
	Walk  WalkOptions
	// VulnDBPath points at a cached Go vuln DB index/modules.json snapshot
	// (see FetchVulnDBModules). When set and present, each repo's go.mod pins
	// are matched against it into RepoReport.VulnFindings. Empty/absent → no-op.
	VulnDBPath string
	// Now supplies the report timestamp; defaults to time.Now (UTC).
	Now func() time.Time
}

// Scan runs the full platform inventory: one bounded parallel walk per repo,
// parity metrics + surface extraction from the walk products, plus drift.
func Scan(ctx context.Context, root string, opts ScanOptions) (PlatformReport, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	start := now()

	repos, err := surfaceinventory.WorkspaceRepos(root, opts.Repos)
	if err != nil {
		return PlatformReport{}, err
	}

	report := PlatformReport{
		GeneratedOn:   start.UTC().Format(time.RFC3339),
		Root:          root,
		SurfaceTotals: map[string]int{},
	}

	vulnDB, vulnErr := LoadVulnDB(opts.VulnDBPath)
	_ = vulnErr // a malformed/absent snapshot degrades to no findings, never fatal

	indexes := walkAll(ctx, root, repos, opts.Walk)
	for i := range indexes {
		idx := &indexes[i]
		repoStart := now()
		parity := CollectParity(idx)
		surfaces := surfaceinventory.ScanFiles(idx.Dir, idx.Repo, idx.Paths, nil)

		rr := RepoReport{
			Repo:       idx.Repo,
			Dir:        idx.Dir,
			Files:      len(idx.Paths),
			Truncated:  idx.Truncated,
			WalkErrors: idx.WalkErrors,
			Parity:     parity,
			Surfaces:   surfaces.Counts,
			MCPRuntime: detectMCPRuntime(idx.Dir),
			ScanMillis: now().Sub(repoStart).Milliseconds(),
		}
		if rr.MCPRuntime.SpecEra != EraNone {
			rr.MCPRuntime.TasksLegacyShape, rr.MCPRuntime.TasksEvidence = detectTasksShape(idx.Dir, idx.Paths)
			rr.CCMeta = detectCCMeta(idx.Dir, idx.Paths)
		}
		if vulnDB != nil {
			rr.VulnFindings = detectVulns(idx.Dir, vulnDB)
		}
		if opts.IncludeSurfaces || opts.Score {
			rr.SurfaceDetail = surfaces.Surfaces
		}
		report.Repos = append(report.Repos, rr)
		report.ViolationCount += len(parity.Violations)
		for k, v := range surfaces.Counts {
			report.SurfaceTotals[k] += v
		}
	}
	sort.Slice(report.Repos, func(i, j int) bool { return report.Repos[i].Repo < report.Repos[j].Repo })

	report.Drift = collectDrift(root, repos)

	if opts.Score {
		sr := Score(report, DefaultScoreWeights)
		report.Scoring = &sr
		if !opts.IncludeSurfaces {
			for i := range report.Repos {
				report.Repos[i].SurfaceDetail = nil
			}
		}
	}
	report.TotalMillis = now().Sub(start).Milliseconds()
	return report, nil
}

// collectDrift compares repos on disk vs workspace/manifest.json vs the docs
// repo catalog (when present). Missing files degrade to empty sections.
func collectDrift(root string, manifestRepos []string) Drift {
	drift := Drift{
		ManifestPath: filepath.Join(root, "workspace", "manifest.json"),
		CatalogPath:  filepath.Join(root, "docs", "inventory", "repo-catalog.json"),
	}

	onDisk := map[string]bool{}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				if _, err := os.Stat(filepath.Join(root, e.Name(), ".git")); err == nil {
					onDisk[e.Name()] = true
				}
			}
		}
	}
	inManifest := map[string]bool{}
	for _, r := range manifestRepos {
		inManifest[r] = true
	}
	inCatalog := map[string]bool{}
	if raw, err := os.ReadFile(drift.CatalogPath); err == nil {
		var cat struct {
			Repos []struct {
				Name          string `json:"name"`
				SourceOfTruth string `json:"source_of_truth"`
			} `json:"repos"`
		}
		if json.Unmarshal(raw, &cat) == nil {
			for _, r := range cat.Repos {
				// Entries whose source of truth is nested inside another
				// repo (absorbed/submodule) are deliberate pointers, not
				// missing checkouts.
				if strings.Contains(r.SourceOfTruth, "/") {
					continue
				}
				inCatalog[r.Name] = true
			}
		}
	}

	for name := range onDisk {
		if !inManifest[name] {
			drift.OnDiskOnly = append(drift.OnDiskOnly, name)
		}
	}
	for name := range inManifest {
		if !onDisk[name] {
			drift.ManifestOnly = append(drift.ManifestOnly, name)
		}
	}
	if len(inCatalog) > 0 {
		for name := range inCatalog {
			if !onDisk[name] && name != ".github" {
				drift.CatalogOnly = append(drift.CatalogOnly, name)
			}
		}
	}
	sort.Strings(drift.OnDiskOnly)
	sort.Strings(drift.ManifestOnly)
	sort.Strings(drift.CatalogOnly)
	return drift
}

// RenderMarkdown renders the platform report as a fleet summary table.
func RenderMarkdown(rep PlatformReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Fleet Inventory\n\nGenerated: %s — root `%s` — %d repos in %dms\n\n",
		rep.GeneratedOn, rep.Root, len(rep.Repos), rep.TotalMillis)

	b.WriteString("| Repo | files | mcp tools | cli | http | canonical skills | violations |\n|---|---|---|---|---|---|---|\n")
	for _, r := range rep.Repos {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %s |\n",
			r.Repo, r.Files,
			r.Surfaces[surfaceinventory.KindMCPTool],
			r.Surfaces[surfaceinventory.KindCLICommand],
			r.Surfaces[surfaceinventory.KindHTTPRoute],
			r.Parity.CanonicalSkills,
			strings.Join(r.Parity.Violations, "; "))
	}
	fmt.Fprintf(&b, "\nTotal violations: %d\n", rep.ViolationCount)

	if len(rep.Drift.OnDiskOnly)+len(rep.Drift.ManifestOnly)+len(rep.Drift.CatalogOnly) > 0 {
		b.WriteString("\n## Drift\n\n")
		if len(rep.Drift.OnDiskOnly) > 0 {
			fmt.Fprintf(&b, "- On disk, not in manifest: %s\n", strings.Join(rep.Drift.OnDiskOnly, ", "))
		}
		if len(rep.Drift.ManifestOnly) > 0 {
			fmt.Fprintf(&b, "- In manifest, missing on disk: %s\n", strings.Join(rep.Drift.ManifestOnly, ", "))
		}
		if len(rep.Drift.CatalogOnly) > 0 {
			fmt.Fprintf(&b, "- In catalog, missing on disk: %s\n", strings.Join(rep.Drift.CatalogOnly, ", "))
		}
	}
	b.WriteString(renderMCPRuntime(rep))
	b.WriteString(renderVulns(rep))
	if rep.Scoring != nil {
		b.WriteString(RenderScoreMarkdown(*rep.Scoring))
	}
	return b.String()
}

// renderVulns lists advisory dependency-vulnerability findings per repo.
func renderVulns(rep PlatformReport) string {
	type row struct {
		repo string
		f    VulnFinding
	}
	var rows []row
	for _, r := range rep.Repos {
		for _, f := range r.VulnFindings {
			rows = append(rows, row{r.Repo, f})
		}
	}
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Dependency vulnerabilities (advisory)\n\n%d version-presence match(es) against the Go vuln DB "+
		"snapshot — advisory triage, NOT reachability (some may be unreachable; govulncheck is the precision lane).\n\n", len(rows))
	b.WriteString("| Repo | module | pinned | vuln | fixed in |\n|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.repo, r.f.Module, r.f.Version, r.f.ID, r.f.Fixed)
	}
	return b.String()
}

// renderMCPRuntime summarizes the fleet's MCP SDK / spec-era distribution.
func renderMCPRuntime(rep PlatformReport) string {
	eraCounts := map[string]int{}
	var mcpRepos []RepoReport
	for _, r := range rep.Repos {
		if r.MCPRuntime.SpecEra == EraNone {
			continue
		}
		eraCounts[r.MCPRuntime.SpecEra]++
		mcpRepos = append(mcpRepos, r)
	}
	if len(mcpRepos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## MCP runtime / spec era\n\n")
	fmt.Fprintf(&b, "Fleet distribution: modern-capable %d, dual %d, legacy-only %d, via-mcpkit %d.\n\n",
		eraCounts[EraModernCapable], eraCounts[EraDual], eraCounts[EraLegacyOnly], eraCounts[EraViaMcpkit])
	b.WriteString("| Repo | spec era | MCP SDK(s) |\n|---|---|---|\n")
	sort.Slice(mcpRepos, func(i, j int) bool { return mcpRepos[i].Repo < mcpRepos[j].Repo })
	var tasksWarnings []string
	for _, r := range mcpRepos {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.Repo, r.MCPRuntime.SpecEra, strings.Join(r.MCPRuntime.SDKs, ", "))
		if w := tasksWarning(r.MCPRuntime); w != "" {
			ev := ""
			if len(r.MCPRuntime.TasksEvidence) > 0 {
				ev = " (" + r.MCPRuntime.TasksEvidence[0] + ")"
			}
			tasksWarnings = append(tasksWarnings, fmt.Sprintf("- **%s**: %s%s", r.Repo, w, ev))
		}
	}
	if len(tasksWarnings) > 0 {
		b.WriteString("\n### Tasks-shape warnings\n\n")
		b.WriteString(strings.Join(tasksWarnings, "\n"))
		b.WriteString("\n")
	}
	b.WriteString(renderCCMeta(mcpRepos))
	return b.String()
}

// renderCCMeta summarizes fleet adoption of Claude Code's _meta tool
// extensions (deferred-loading opt-out, output-size, consent-prompt).
func renderCCMeta(mcpRepos []RepoReport) string {
	var adopters []RepoReport
	for _, r := range mcpRepos {
		if r.CCMeta.Any() {
			adopters = append(adopters, r)
		}
	}
	var b strings.Builder
	b.WriteString("\n### Claude Code _meta extensions\n\n")
	if len(adopters) == 0 {
		fmt.Fprintf(&b, "No fleet server sets Claude Code's per-tool _meta extensions (anthropic/alwaysLoad, "+
			"maxResultSizeChars, requiresUserInteraction). Tool-search / deferred loading runs on Claude Code's "+
			"defaults; the large servers (%s) could opt their hottest tools into alwaysLoad or raise "+
			"maxResultSizeChars for big-output tools.\n", "hg-mcp/mesmer/jellyfin-mcp-deluxe/secretstudios")
		return b.String()
	}
	b.WriteString("| Repo | alwaysLoad | maxResultChars | skipRequiresUser | literalKeys |\n|---|---|---|---|---|\n")
	for _, r := range adopters {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", r.Repo,
			r.CCMeta.AlwaysLoad, r.CCMeta.MaxResultChars, r.CCMeta.SkipRequiresUser, r.CCMeta.LiteralMetaKeys)
	}
	var warns []string
	for _, r := range adopters {
		if r.CCMeta.Warning != "" {
			warns = append(warns, fmt.Sprintf("- **%s**: %s", r.Repo, r.CCMeta.Warning))
		}
	}
	if len(warns) > 0 {
		b.WriteString("\n" + strings.Join(warns, "\n") + "\n")
	}
	return b.String()
}
