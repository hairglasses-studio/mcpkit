package fleetinventory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

// Scoring layer: turns a PlatformReport into per-repo 0-100 quality scores
// and a namespace-level cross-repo duplication view. Dimensions a repo lacks
// data for stay nil and the composite renormalizes over what was measured
// (DataConfidence reports the measured weight fraction); a truncated walk
// caps confidence so an incomplete scan cannot masquerade as a full one.

// ScoreWeights are the per-dimension weights of the composite.
type ScoreWeights struct {
	DescriptionCoverage float64 `json:"description_coverage"`
	NamingDiscipline    float64 `json:"naming_discipline"`
	Duplication         float64 `json:"duplication"`
	ViolationBurden     float64 `json:"violation_burden"`
	SizeOutlier         float64 `json:"size_outlier"`
	DeclaredGap         float64 `json:"declared_gap"`
}

// DefaultScoreWeights mirror the lane-scoring prototype.
var DefaultScoreWeights = ScoreWeights{
	DescriptionCoverage: 0.20,
	NamingDiscipline:    0.15,
	Duplication:         0.20,
	ViolationBurden:     0.20,
	SizeOutlier:         0.15,
	DeclaredGap:         0.10,
}

// ScoreDimensions holds per-dimension scores; nil = not measured.
type ScoreDimensions struct {
	DescriptionCoverage *int `json:"description_coverage,omitempty"`
	NamingDiscipline    *int `json:"naming_discipline,omitempty"`
	Duplication         *int `json:"duplication,omitempty"`
	ViolationBurden     int  `json:"violation_burden"`
	SizeOutlier         *int `json:"size_outlier,omitempty"`
	DeclaredGap         *int `json:"declared_gap,omitempty"`
}

// RepoScore is one repo's scored row.
type RepoScore struct {
	Repo            string          `json:"repo"`
	Dims            ScoreDimensions `json:"dims"`
	Composite       *float64        `json:"composite_score,omitempty"`
	DataConfidence  float64         `json:"data_confidence"`
	ImpactWeight    float64         `json:"impact_weight"`
	RoadmapPriority *float64        `json:"roadmap_priority,omitempty"`
	Notes           []string        `json:"notes,omitempty"`
}

// NamespaceScore aggregates one tool-name prefix across the fleet.
type NamespaceScore struct {
	Namespace           string   `json:"namespace"`
	ToolCount           int      `json:"tool_count"`
	RepoSpan            int      `json:"repo_span"`
	Repos               []string `json:"repos"`
	DescriptionCoverage int      `json:"description_coverage"`
	CrossRepoDuplicate  bool     `json:"cross_repo_duplicate"`
}

// ScoreReport is the scoring section of a PlatformReport.
type ScoreReport struct {
	Repos      []RepoScore      `json:"repos"`
	Namespaces []NamespaceScore `json:"namespaces"`
	Weights    ScoreWeights     `json:"weights"`
}

// scaffoldNames are SDK-example boilerplate, excluded from naming and
// duplication signals.
var scaffoldNames = map[string]bool{
	"greet": true, "echo": true, "ping": true, "test": true, "hello": true,
	"foo": true, "bar": true, "dummy": true, "sample": true,
}

func isScaffoldName(name string) bool {
	lower := strings.ToLower(name)
	return scaffoldNames[lower] ||
		strings.HasPrefix(lower, "test_") ||
		strings.HasPrefix(lower, "example_") ||
		strings.HasPrefix(lower, "demo_")
}

var mcpKinds = map[string]bool{
	surfaceinventory.KindMCPTool:     true,
	surfaceinventory.KindMCPResource: true,
	surfaceinventory.KindMCPPrompt:   true,
}

// manifestImpact is the subset of workspace/manifest.json scoring reads.
type manifestImpact struct {
	Repos []struct {
		Name      string `json:"name"`
		Scope     string `json:"scope"`
		Lifecycle string `json:"lifecycle"`
	} `json:"repos"`
}

func impactWeights(root string) map[string]float64 {
	out := map[string]float64{}
	raw, err := os.ReadFile(filepath.Join(root, "workspace", "manifest.json"))
	if err != nil {
		return out
	}
	var m manifestImpact
	if json.Unmarshal(raw, &m) != nil {
		return out
	}
	lifecycleW := map[string]float64{
		"active": 1.0, "never-publish": 1.0, "publish-mirror": 0.8,
		"maintenance-only": 0.6, "deprecated": 0.3, "archived": 0.2,
	}
	scopeW := map[string]float64{
		"active_operator": 1.0, "active_first_party": 1.0, "compatibility_only": 0.3,
	}
	for _, r := range m.Repos {
		lw, ok := lifecycleW[r.Lifecycle]
		if !ok {
			lw = 1.0
		}
		sw, ok := scopeW[r.Scope]
		if !ok {
			sw = 1.0
		}
		out[r.Name] = lw * sw
	}
	return out
}

// Score computes the scoring section for a PlatformReport. Repos scanned
// without surface detail get only the always-available dimensions.
func Score(rep PlatformReport, weights ScoreWeights) ScoreReport {
	sr := ScoreReport{Weights: weights}
	impacts := impactWeights(rep.Root)

	// Cross-repo duplicate name index (tools only, scaffold excluded).
	nameRepos := map[string]map[string]bool{}
	for _, r := range rep.Repos {
		for _, s := range r.SurfaceDetail {
			if s.Kind != surfaceinventory.KindMCPTool || isScaffoldName(s.Name) {
				continue
			}
			if nameRepos[s.Name] == nil {
				nameRepos[s.Name] = map[string]bool{}
			}
			nameRepos[s.Name][r.Repo] = true
		}
	}

	// Fleet median/MAD of nonzero tool counts for the size dimension.
	var toolCounts []float64
	for _, r := range rep.Repos {
		if n := r.Surfaces[surfaceinventory.KindMCPTool]; n > 0 {
			toolCounts = append(toolCounts, float64(n))
		}
	}
	med, mad := medianMAD(toolCounts)

	for _, r := range rep.Repos {
		score := scoreRepo(r, weights, nameRepos, med, mad)
		if w, ok := impacts[r.Repo]; ok {
			score.ImpactWeight = w
		} else {
			score.ImpactWeight = 1.0
		}
		if score.Composite != nil {
			p := (100 - *score.Composite) * score.ImpactWeight
			score.RoadmapPriority = &p
		}
		sr.Repos = append(sr.Repos, score)
	}
	sort.Slice(sr.Repos, func(i, j int) bool {
		pi, pj := 0.0, 0.0
		if sr.Repos[i].RoadmapPriority != nil {
			pi = *sr.Repos[i].RoadmapPriority
		}
		if sr.Repos[j].RoadmapPriority != nil {
			pj = *sr.Repos[j].RoadmapPriority
		}
		if pi != pj {
			return pi > pj
		}
		return sr.Repos[i].Repo < sr.Repos[j].Repo
	})

	sr.Namespaces = namespaceScores(rep)
	return sr
}

func scoreRepo(r RepoReport, w ScoreWeights, nameRepos map[string]map[string]bool, med, mad float64) RepoScore {
	rs := RepoScore{Repo: r.Repo}
	hasDetail := len(r.SurfaceDetail) > 0

	// violation_burden — always measured.
	burden := 0
	for _, v := range r.Parity.Violations {
		switch {
		case strings.HasPrefix(v, "retired mirror"), strings.HasPrefix(v, "missing AGENTS.md"):
			burden += 3
		case strings.Contains(v, "[profiles.*]"):
			burden += 2
		default:
			burden++
		}
	}
	rs.Dims.ViolationBurden = clamp(100 - 15*burden)

	measured := w.ViolationBurden
	total := w.DescriptionCoverage + w.NamingDiscipline + w.Duplication + w.ViolationBurden + w.SizeOutlier + w.DeclaredGap
	sum := float64(rs.Dims.ViolationBurden) * w.ViolationBurden

	if hasDetail {
		if d := descCoverage(r.SurfaceDetail); d != nil {
			rs.Dims.DescriptionCoverage = d
			measured += w.DescriptionCoverage
			sum += float64(*d) * w.DescriptionCoverage
		}
		if n := namingDiscipline(r.SurfaceDetail); n != nil {
			rs.Dims.NamingDiscipline = n
			measured += w.NamingDiscipline
			sum += float64(*n) * w.NamingDiscipline
		}
		if d := duplicationScore(r, nameRepos); d != nil {
			rs.Dims.Duplication = d
			measured += w.Duplication
			sum += float64(*d) * w.Duplication
		}
	}
	if n := r.Surfaces[surfaceinventory.KindMCPTool]; n > 0 && mad > 0 {
		z := math.Abs(float64(n)-med) / mad
		v := clamp(int(math.Round(100 - 12*z)))
		rs.Dims.SizeOutlier = &v
		measured += w.SizeOutlier
		sum += float64(v) * w.SizeOutlier
		if z > 5 {
			rs.Notes = append(rs.Notes, fmt.Sprintf("size outlier: %d tools (fleet median %.0f)", n, med))
		}
	}
	if g := declaredGap(r); g != nil {
		rs.Dims.DeclaredGap = g
		measured += w.DeclaredGap
		sum += float64(*g) * w.DeclaredGap
	}

	if measured > 0 {
		c := sum / measured
		rs.Composite = &c
	}
	rs.DataConfidence = measured / total
	if r.Truncated {
		// An incomplete walk cannot claim full confidence.
		if rs.DataConfidence > 0.5 {
			rs.DataConfidence = 0.5
		}
		rs.Notes = append(rs.Notes, "walk truncated at file cap — scores unreliable")
	}
	return rs
}

func descCoverage(surfaces []surfaceinventory.Surface) *int {
	total, described := 0, 0
	for _, s := range surfaces {
		if !mcpKinds[s.Kind] {
			continue
		}
		total++
		if strings.TrimSpace(s.Description) != "" {
			described++
		}
	}
	if total == 0 {
		return nil
	}
	v := clamp(100 * described / total)
	return &v
}

func namingDiscipline(surfaces []surfaceinventory.Surface) *int {
	total, flagged, snake, camel := 0, 0, 0, 0
	for _, s := range surfaces {
		if !mcpKinds[s.Kind] {
			continue
		}
		total++
		name := s.Name
		if len(name) <= 1 || strings.ContainsAny(name, " \t()") || isScaffoldName(name) {
			flagged++
			continue
		}
		if strings.Contains(name, "_") || strings.ToLower(name) == name {
			snake++
		} else {
			camel++
		}
	}
	if total == 0 {
		return nil
	}
	v := 100 * (total - flagged) / total
	styled := snake + camel
	if styled > 0 {
		dominant := snake
		if camel > dominant {
			dominant = camel
		}
		if float64(dominant)/float64(styled) < 0.9 {
			v -= 15
		}
	}
	c := clamp(v)
	return &c
}

func duplicationScore(r RepoReport, nameRepos map[string]map[string]bool) *int {
	names := map[string]int{}
	total := 0
	crossDup := 0
	for _, s := range r.SurfaceDetail {
		if s.Kind != surfaceinventory.KindMCPTool || isScaffoldName(s.Name) {
			continue
		}
		total++
		names[s.Name]++
		if len(nameRepos[s.Name]) >= 2 {
			crossDup++
		}
	}
	if total == 0 {
		return nil
	}
	intra := 1 - float64(len(names))/float64(total)
	cross := float64(crossDup) / float64(total)
	v := clamp(int(math.Round(100 * (1 - 0.5*intra - 0.5*cross))))
	return &v
}

// declaredGap trusts only a machine-readable tool_count in the repo's
// .well-known/mcp.json (prose "N tools" claims in READMEs are too noisy to
// score against — see the lane-scoring gap analysis).
func declaredGap(r RepoReport) *int {
	raw, err := os.ReadFile(filepath.Join(repoDir(r), ".well-known", "mcp.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		ToolCount int `json:"tool_count"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.ToolCount <= 0 {
		return nil
	}
	scanned := r.Surfaces[surfaceinventory.KindMCPTool]
	maxN := doc.ToolCount
	if scanned > maxN {
		maxN = scanned
	}
	if maxN == 0 {
		return nil
	}
	gap := math.Abs(float64(doc.ToolCount-scanned)) / float64(maxN)
	v := clamp(int(math.Round(100 - 200*gap)))
	return &v
}

func repoDir(r RepoReport) string { return r.Dir }

func namespaceScores(rep PlatformReport) []NamespaceScore {
	type agg struct {
		count, described int
		repos            map[string]bool
	}
	byNS := map[string]*agg{}
	for _, r := range rep.Repos {
		for _, s := range r.SurfaceDetail {
			if s.Kind != surfaceinventory.KindMCPTool || isScaffoldName(s.Name) {
				continue
			}
			ns := s.Name
			if i := strings.Index(ns, "_"); i > 0 {
				ns = ns[:i]
			}
			a := byNS[ns]
			if a == nil {
				a = &agg{repos: map[string]bool{}}
				byNS[ns] = a
			}
			a.count++
			a.repos[r.Repo] = true
			if strings.TrimSpace(s.Description) != "" {
				a.described++
			}
		}
	}
	var out []NamespaceScore
	for ns, a := range byNS {
		if a.count < 2 {
			continue
		}
		repos := make([]string, 0, len(a.repos))
		for r := range a.repos {
			repos = append(repos, r)
		}
		sort.Strings(repos)
		out = append(out, NamespaceScore{
			Namespace:           ns,
			ToolCount:           a.count,
			RepoSpan:            len(repos),
			Repos:               repos,
			DescriptionCoverage: clamp(100 * a.described / a.count),
			CrossRepoDuplicate:  len(repos) >= 2,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RepoSpan != out[j].RepoSpan {
			return out[i].RepoSpan > out[j].RepoSpan
		}
		if out[i].ToolCount != out[j].ToolCount {
			return out[i].ToolCount > out[j].ToolCount
		}
		return out[i].Namespace < out[j].Namespace
	})
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

func medianMAD(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	med := sorted[len(sorted)/2]
	devs := make([]float64, len(sorted))
	for i, v := range sorted {
		devs[i] = math.Abs(v - med)
	}
	sort.Float64s(devs)
	mad := devs[len(devs)/2]
	if mad == 0 {
		mad = 1
	}
	return med, mad
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// RenderScoreMarkdown renders the scoreboard section.
func RenderScoreMarkdown(sr ScoreReport) string {
	var b strings.Builder
	b.WriteString("\n## Quality Scoreboard\n\n| Repo | composite | confidence | desc | naming | dup | violations | size | priority | notes |\n|---|---|---|---|---|---|---|---|---|---|\n")
	dim := func(p *int) string {
		if p == nil {
			return "—"
		}
		return fmt.Sprintf("%d", *p)
	}
	for _, r := range sr.Repos {
		comp, prio := "—", "—"
		if r.Composite != nil {
			comp = fmt.Sprintf("%.1f", *r.Composite)
		}
		if r.RoadmapPriority != nil {
			prio = fmt.Sprintf("%.1f", *r.RoadmapPriority)
		}
		fmt.Fprintf(&b, "| %s | %s | %.2f | %s | %s | %s | %d | %s | %s | %s |\n",
			r.Repo, comp, r.DataConfidence,
			dim(r.Dims.DescriptionCoverage), dim(r.Dims.NamingDiscipline), dim(r.Dims.Duplication),
			r.Dims.ViolationBurden, dim(r.Dims.SizeOutlier), prio, strings.Join(r.Notes, "; "))
	}
	if len(sr.Namespaces) > 0 {
		b.WriteString("\n### Cross-repo namespaces (top by span)\n\n| Namespace | tools | repos |\n|---|---|---|\n")
		for i, ns := range sr.Namespaces {
			if i >= 15 || !ns.CrossRepoDuplicate {
				continue
			}
			fmt.Fprintf(&b, "| %s | %d | %s |\n", ns.Namespace, ns.ToolCount, strings.Join(ns.Repos, ", "))
		}
	}
	return b.String()
}
