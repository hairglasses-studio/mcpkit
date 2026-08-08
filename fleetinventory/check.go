package fleetinventory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CI gating for the fleet inventory. Following the generate/gate split that
// grype/syft/Scorecard use: the scan is pure data (fleet_inventory_scan/
// _score never fail); a SEPARATE check step diffs a fresh scored scan against a
// committed baseline and fails on regression. The baseline is a compact,
// stable, diffable projection of the scored report — small enough to commit and
// review, so it doubles as the "state we accept" record.
//
// Regression predicates (all derived from one baseline, not separate
// mechanisms): a repo's composite drops more than the allowed delta, a repo
// gains a baseline violation / security finding, or a repo becomes Tasks-shape
// non-conformant. New repos are reported as info, never a failure.

// DefaultCompositeDrop is the allowed composite regression before it fails.
const DefaultCompositeDrop = 5.0

// BaselineRepo is one repo's accepted state.
type BaselineRepo struct {
	Composite          *float64 `json:"composite,omitempty"`
	Violations         int      `json:"violations"`
	SecurityFindings   int      `json:"security_findings"`
	TasksNonConformant bool     `json:"tasks_non_conformant"`
}

// CheckBaseline is the committed accepted-state file.
type CheckBaseline struct {
	GeneratedOn string                  `json:"generated_on"`
	Root        string                  `json:"root"`
	Repos       map[string]BaselineRepo `json:"repos"`
}

// tasksNonConformant reports whether a repo uses the removed-from-core Tasks
// shape on a modern-capable/dual server (the NON-CONFORMANT severity, not the
// legacy-only migration-hazard).
func tasksNonConformant(r RepoReport) bool {
	if !r.MCPRuntime.TasksLegacyShape {
		return false
	}
	return r.MCPRuntime.SpecEra == EraModernCapable || r.MCPRuntime.SpecEra == EraDual
}

// BaselineFromReport projects a scored report into a committable baseline.
// The report MUST have been produced with Score:true (Scoring non-nil).
func BaselineFromReport(rep PlatformReport) CheckBaseline {
	b := CheckBaseline{GeneratedOn: rep.GeneratedOn, Root: rep.Root, Repos: map[string]BaselineRepo{}}
	sec := map[string]int{}
	comp := map[string]*float64{}
	if rep.Scoring != nil {
		for _, s := range rep.Scoring.Repos {
			sec[s.Repo] = len(s.SecurityFindings)
			comp[s.Repo] = s.Composite
		}
	}
	for _, r := range rep.Repos {
		b.Repos[r.Repo] = BaselineRepo{
			Composite:          comp[r.Repo],
			Violations:         len(r.Parity.Violations),
			SecurityFindings:   sec[r.Repo],
			TasksNonConformant: tasksNonConformant(r),
		}
	}
	return b
}

// Marshal renders the baseline as stable indented JSON for committing.
func (b CheckBaseline) Marshal() ([]byte, error) { return json.MarshalIndent(b, "", "  ") }

// ParseBaseline loads a committed baseline.
func ParseBaseline(raw []byte) (CheckBaseline, error) {
	var b CheckBaseline
	err := json.Unmarshal(raw, &b)
	return b, err
}

// CheckResult is the gate outcome.
type CheckResult struct {
	Passed       bool     `json:"passed"`
	Regressions  []string `json:"regressions,omitempty"`
	NewRepos     []string `json:"new_repos,omitempty"`
	RemovedRepos []string `json:"removed_repos,omitempty"`
	Checked      int      `json:"checked"`
}

// Check diffs a fresh scored report against a baseline. compositeDrop is the
// allowed composite regression (0 → DefaultCompositeDrop).
func Check(rep PlatformReport, base CheckBaseline, compositeDrop float64) CheckResult {
	if compositeDrop <= 0 {
		compositeDrop = DefaultCompositeDrop
	}
	res := CheckResult{Passed: true}
	seen := map[string]bool{}
	for _, r := range rep.Repos {
		seen[r.Repo] = true
		bl, ok := base.Repos[r.Repo]
		if !ok {
			res.NewRepos = append(res.NewRepos, r.Repo)
			continue
		}
		res.Checked++

		var comp *float64
		var secCount int
		if rep.Scoring != nil {
			for _, s := range rep.Scoring.Repos {
				if s.Repo == r.Repo {
					comp = s.Composite
					secCount = len(s.SecurityFindings)
					break
				}
			}
		}
		if comp != nil && bl.Composite != nil && *comp < *bl.Composite-compositeDrop {
			res.Regressions = append(res.Regressions,
				fmt.Sprintf("%s: composite %.1f → %.1f (drop > %.1f)", r.Repo, *bl.Composite, *comp, compositeDrop))
		}
		if v := len(r.Parity.Violations); v > bl.Violations {
			res.Regressions = append(res.Regressions,
				fmt.Sprintf("%s: baseline violations %d → %d (new: %s)", r.Repo, bl.Violations, v, strings.Join(r.Parity.Violations, "; ")))
		}
		if secCount > bl.SecurityFindings {
			res.Regressions = append(res.Regressions,
				fmt.Sprintf("%s: security findings %d → %d", r.Repo, bl.SecurityFindings, secCount))
		}
		if tasksNonConformant(r) && !bl.TasksNonConformant {
			res.Regressions = append(res.Regressions,
				fmt.Sprintf("%s: became Tasks-shape NON-CONFORMANT (modern server using removed-from-core Tasks)", r.Repo))
		}
	}
	for name := range base.Repos {
		if !seen[name] {
			res.RemovedRepos = append(res.RemovedRepos, name)
		}
	}
	sort.Strings(res.Regressions)
	sort.Strings(res.NewRepos)
	sort.Strings(res.RemovedRepos)
	res.Passed = len(res.Regressions) == 0
	return res
}

// RenderCheck formats a check result for CI logs.
func RenderCheck(res CheckResult) string {
	var b strings.Builder
	status := "PASS"
	if !res.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "fleet inventory check: %s (%d repos checked)\n", status, res.Checked)
	if len(res.Regressions) > 0 {
		b.WriteString("\nRegressions:\n")
		for _, r := range res.Regressions {
			fmt.Fprintf(&b, "  - %s\n", r)
		}
	}
	if len(res.NewRepos) > 0 {
		fmt.Fprintf(&b, "\nNew repos (not a failure): %s\n", strings.Join(res.NewRepos, ", "))
	}
	if len(res.RemovedRepos) > 0 {
		fmt.Fprintf(&b, "Removed repos: %s\n", strings.Join(res.RemovedRepos, ", "))
	}
	return b.String()
}
