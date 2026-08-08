package fleetinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// Dependency vulnerability advisory. There is no build-free govulncheck mode
// (source/binary modes both need a type-checked build), so this does NOT
// reimplement reachability analysis. Instead it does version-PRESENCE matching:
// parse each repo's go.mod dependency pins and compare them against an offline
// snapshot of the Go vulnerability database's index/modules.json
// (https://vuln.go.dev/index/modules.json — fetched out-of-band, see
// FetchVulnDBModules). A pin is flagged if its version is below a `fixed`
// version the DB records for that module.
//
// This is ADVISORY / triage, not a precision gate: version-presence over-flags
// vulnerable-but-UNREACHABLE transitive deps that a full `govulncheck` run
// scores clean (keep govulncheck as the precision hard-fail lane). Its value is
// the fleet-wide MAP — which of 30+ repos pin which vulnerable version — that no
// per-repo run gives. Not a scored dimension (unreachable vulns shouldn't tank a
// repo's composite); surfaced as an informational section.
//
// Accuracy: Go module paths encode the major version (…/v2), so all versions of
// one module path share a major and a plain `version < fixed` semver compare per
// module path is correct. The DB entries observed are `introduced:0 → fixed`, so
// version-below-fixed is affected; a vuln introduced mid-range (rare) would
// over-flag — acceptable for an advisory.

// VulnEntry is one advisory for a module from the DB index.
type VulnEntry struct {
	ID    string `json:"id"`
	Fixed string `json:"fixed"`
}

// VulnDB is the module→advisories index (from index/modules.json).
type VulnDB struct {
	byModule map[string][]VulnEntry
}

// vulnModulesEntry mirrors one element of index/modules.json.
type vulnModulesEntry struct {
	Path  string      `json:"path"`
	Vulns []VulnEntry `json:"vulns"`
}

// LoadVulnDB reads a Go vuln DB index/modules.json snapshot. Returns nil (no
// error) when the path is empty or absent — the dimension then no-ops.
func LoadVulnDB(path string) (*VulnDB, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []vulnModulesEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	db := &VulnDB{byModule: make(map[string][]VulnEntry, len(entries))}
	for _, e := range entries {
		if e.Path != "" && len(e.Vulns) > 0 {
			db.byModule[e.Path] = e.Vulns
		}
	}
	return db, nil
}

// VulnFinding is one flagged dependency pin.
type VulnFinding struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	ID      string `json:"id"`
	Fixed   string `json:"fixed"`
}

// normalizeSemver adds the leading "v" the vuln DB omits and canonicalizes.
func normalizeSemver(v string) string {
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

// matchDeps flags pins below a recorded fixed version.
func matchDeps(deps map[string]string, db *VulnDB) []VulnFinding {
	if db == nil {
		return nil
	}
	var out []VulnFinding
	for mod, ver := range deps {
		entries, ok := db.byModule[mod]
		if !ok {
			continue
		}
		dv := normalizeSemver(ver)
		if !semver.IsValid(dv) {
			continue
		}
		for _, e := range entries {
			fv := normalizeSemver(e.Fixed)
			if fv == "" || !semver.IsValid(fv) {
				continue
			}
			if semver.Compare(dv, fv) < 0 {
				out = append(out, VulnFinding{Module: mod, Version: ver, ID: e.ID, Fixed: e.Fixed})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// parseModDeps returns the effective module→version pins from every go.mod
// under dir (bounded), applying versioned replace directives. Local-path
// replaces drop the module from vuln matching (the pinned version isn't used).
func parseModDeps(dir string) map[string]string {
	deps := map[string]string{}
	base := filepath.Clean(dir)
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir {
				name := d.Name()
				if strings.HasPrefix(name, ".") || prunedDirs[name] {
					return filepath.SkipDir
				}
				if rel, e := filepath.Rel(base, path); e == nil && strings.Count(rel, string(filepath.Separator)) >= 3 {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		f, e := modfile.Parse(path, raw, nil)
		if e != nil {
			return nil
		}
		for _, r := range f.Require {
			if cur, ok := deps[r.Mod.Path]; !ok || semver.Compare(normalizeSemver(r.Mod.Version), normalizeSemver(cur)) > 0 {
				deps[r.Mod.Path] = r.Mod.Version
			}
		}
		for _, rp := range f.Replace {
			if rp.New.Version == "" { // local-path replace — version not used
				delete(deps, rp.Old.Path)
				continue
			}
			deps[rp.Old.Path] = rp.New.Version
		}
		return nil
	})
	return deps
}

// detectVulns parses a repo's go.mod deps and matches them against the DB.
func detectVulns(dir string, db *VulnDB) []VulnFinding {
	if db == nil {
		return nil
	}
	return matchDeps(parseModDeps(dir), db)
}
