package fleetinventory

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type scanInput struct {
	Root            string   `json:"root" jsonschema:"required,description=Workspace root directory containing the repos"`
	Repos           []string `json:"repos,omitempty" jsonschema:"description=Repo names to scan. Omit to use workspace/manifest.json or .git discovery."`
	IncludeSurfaces bool     `json:"include_surfaces,omitempty" jsonschema:"description=Include per-surface detail rows (large output)"`
}

type scanOutput struct {
	Report PlatformReport `json:"report"`
}

type reportInput struct {
	Root  string   `json:"root" jsonschema:"required,description=Workspace root directory containing the repos"`
	Repos []string `json:"repos,omitempty" jsonschema:"description=Repo names to scan. Omit for full workspace."`
}

type reportOutput struct {
	Markdown string `json:"markdown"`
}

type baselineOutput struct {
	BaselineJSON string `json:"baseline_json"`
}

type checkInput struct {
	Root          string   `json:"root" jsonschema:"required,description=Workspace root directory"`
	Repos         []string `json:"repos,omitempty" jsonschema:"description=Repo names to scan. Omit for full workspace."`
	BaselineJSON  string   `json:"baseline_json" jsonschema:"required,description=The committed baseline JSON (from fleet_inventory_baseline) to diff against"`
	CompositeDrop float64  `json:"composite_drop,omitempty" jsonschema:"description=Allowed composite regression before failing (default 5.0)"`
}

type checkOutput struct {
	Result CheckResult `json:"result"`
	Report string      `json:"report"`
}

type module struct{}

// Module returns the fleet-inventory platform ToolModule:
//
//	reg.RegisterModule(fleetinventory.Module())
func Module() registry.ToolModule { return &module{} }

func (m *module) Name() string { return "fleet_inventory" }
func (m *module) Description() string {
	return "Fleet inventory platform: one bounded parallel walk per repo feeding provider-parity metrics, code-surface extraction, and workspace drift detection"
}

func (m *module) Tools() []registry.ToolDefinition {
	scan := handler.TypedHandler[scanInput, scanOutput](
		"fleet_inventory_scan",
		"Run the full fleet inventory: per-repo provider-parity metrics (instruction files, retired mirrors, skills, MCP config sources), code-surface counts, baseline violations, and disk/manifest/catalog drift. Bounded and parallel; errors are recorded per repo, never fatal.",
		func(ctx context.Context, in scanInput) (scanOutput, error) {
			rep, err := Scan(ctx, in.Root, ScanOptions{Repos: in.Repos, IncludeSurfaces: in.IncludeSurfaces})
			if err != nil {
				return scanOutput{}, err
			}
			return scanOutput{Report: rep}, nil
		},
	)
	scan.Category = "audit"
	scan.SearchTerms = []string{"fleet inventory", "parity audit", "workspace drift", "baseline violations", "surface counts"}

	report := handler.TypedHandler[reportInput, reportOutput](
		"fleet_inventory_report",
		"Render the fleet inventory as a Markdown summary (per-repo files/surfaces/skills/violations table + drift section).",
		func(ctx context.Context, in reportInput) (reportOutput, error) {
			rep, err := Scan(ctx, in.Root, ScanOptions{Repos: in.Repos})
			if err != nil {
				return reportOutput{}, err
			}
			return reportOutput{Markdown: RenderMarkdown(rep)}, nil
		},
	)
	report.Category = "audit"
	report.SearchTerms = []string{"fleet report", "inventory markdown", "parity summary"}

	score := handler.TypedHandler[reportInput, scanOutput](
		"fleet_inventory_score",
		"Run the fleet inventory with the quality scoreboard: per-repo 0-100 composite scores (description coverage, naming discipline, duplication, violation burden, size outliers, declared-count gap) with data-confidence and lifecycle-weighted roadmap priority, plus cross-repo namespace duplication.",
		func(ctx context.Context, in reportInput) (scanOutput, error) {
			rep, err := Scan(ctx, in.Root, ScanOptions{Repos: in.Repos, Score: true})
			if err != nil {
				return scanOutput{}, err
			}
			return scanOutput{Report: rep}, nil
		},
	)
	score.Category = "audit"
	score.SearchTerms = []string{"quality score", "cleanup roadmap", "repo scoreboard", "namespace duplication"}

	baseline := handler.TypedHandler[reportInput, baselineOutput](
		"fleet_inventory_baseline",
		"Emit a committable baseline (compact per-repo composite / violation / security-finding / Tasks-conformance projection of a scored scan) for use with fleet_inventory_check as a CI gate.",
		func(ctx context.Context, in reportInput) (baselineOutput, error) {
			rep, err := Scan(ctx, in.Root, ScanOptions{Repos: in.Repos, Score: true})
			if err != nil {
				return baselineOutput{}, err
			}
			raw, err := BaselineFromReport(rep).Marshal()
			if err != nil {
				return baselineOutput{}, err
			}
			return baselineOutput{BaselineJSON: string(raw)}, nil
		},
	)
	baseline.Category = "audit"
	baseline.SearchTerms = []string{"baseline", "ci gate", "accepted state", "commit baseline"}

	check := handler.TypedHandler[checkInput, checkOutput](
		"fleet_inventory_check",
		"CI gate: scan the workspace and diff against a committed baseline (from fleet_inventory_baseline). Fails (passed=false) when a repo's composite drops beyond the allowed delta, gains a baseline violation or security finding, or becomes Tasks-shape non-conformant. New repos are reported, not failed.",
		func(ctx context.Context, in checkInput) (checkOutput, error) {
			base, err := ParseBaseline([]byte(in.BaselineJSON))
			if err != nil {
				return checkOutput{}, fmt.Errorf("parse baseline: %w", err)
			}
			rep, err := Scan(ctx, in.Root, ScanOptions{Repos: in.Repos, Score: true})
			if err != nil {
				return checkOutput{}, err
			}
			res := Check(rep, base, in.CompositeDrop)
			return checkOutput{Result: res, Report: RenderCheck(res)}, nil
		},
	)
	check.Category = "audit"
	check.SearchTerms = []string{"ci gate", "regression check", "fail on regression", "baseline diff"}

	return []registry.ToolDefinition{scan, report, score, baseline, check}
}
