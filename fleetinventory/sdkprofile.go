package fleetinventory

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MCP runtime profiling: which MCP SDK(s) a repo depends on and therefore
// which wire-protocol era its server code targets. The 2026-07-28 spec is a
// breaking rewrite ("Modern"/stateless: no `initialize`, `server/discover`,
// MRTR); 2025-11-25 ("Legacy") is what most of the fleet still speaks. This
// is an INFORMATIONAL fleet-map signal, not a scored penalty — running the
// ecosystem-standard legacy shape is not a defect.
//
// Detection is a static go.mod parse (bounded walk). Caveat per the SDK
// research: an official-go-sdk >=v1.7.0 dependency means Modern-CAPABLE, not
// that the code actually speaks Modern (the SDK back-compats to older eras);
// authoritative era detection needs a live probe (deferred to the
// live-interrogation variant).

// Spec eras.
const (
	EraModernCapable = "modern-capable" // official go-sdk >= v1.7.0
	EraDual          = "dual"           // both official (>=1.7.0) and mark3labs present
	EraLegacyOnly    = "legacy-only"    // mark3labs/mcp-go, or official < v1.7.0
	EraViaMcpkit     = "via-mcpkit"     // depends on mcpkit, no direct SDK require
	EraNone          = ""               // no MCP SDK dependency (non-Go or non-MCP)
)

const (
	modOfficial = "github.com/modelcontextprotocol/go-sdk"
	modMark3    = "github.com/mark3labs/mcp-go"
	modMcpkit   = "github.com/hairglasses-studio/mcpkit"
)

// MCPRuntime is a repo's MCP SDK/era profile.
type MCPRuntime struct {
	SDKs    []string `json:"sdks,omitempty"` // "module vX.Y.Z" for each MCP SDK required
	SpecEra string   `json:"spec_era,omitempty"`
}

// detectMCPRuntime parses the go.mod files under dir (bounded) and classifies
// the repo's MCP SDK dependency + spec era.
func detectMCPRuntime(dir string) MCPRuntime {
	versions := map[string]string{} // module -> version (highest seen)
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
		parseGoModRequires(path, versions)
		return nil
	})

	rt := MCPRuntime{}
	for _, m := range []string{modOfficial, modMark3, modMcpkit} {
		if v, ok := versions[m]; ok {
			rt.SDKs = append(rt.SDKs, shortModule(m)+" "+v)
		}
	}
	sort.Strings(rt.SDKs)

	officialV, hasOfficial := versions[modOfficial]
	_, hasMark3 := versions[modMark3]
	_, hasMcpkit := versions[modMcpkit]
	officialModern := hasOfficial && semverAtLeast(officialV, 1, 7)

	switch {
	case officialModern && hasMark3:
		rt.SpecEra = EraDual
	case officialModern:
		rt.SpecEra = EraModernCapable
	case hasMark3 || hasOfficial:
		rt.SpecEra = EraLegacyOnly
	case hasMcpkit:
		rt.SpecEra = EraViaMcpkit
	default:
		rt.SpecEra = EraNone
	}
	return rt
}

// parseGoModRequires reads require lines (single and block form) for the MCP
// modules of interest, recording the highest version seen per module.
func parseGoModRequires(path string, out map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inBlock := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			considerRequire(strings.TrimPrefix(line, "require "), out)
		case inBlock:
			considerRequire(line, out)
		}
	}
}

func considerRequire(spec string, out map[string]string) {
	fields := strings.Fields(spec)
	if len(fields) < 2 {
		return
	}
	mod, ver := fields[0], fields[1]
	if mod != modOfficial && mod != modMark3 && mod != modMcpkit {
		return
	}
	if cur, ok := out[mod]; !ok || semverGreater(ver, cur) {
		out[mod] = ver
	}
}

func shortModule(m string) string {
	if i := strings.LastIndex(m, "/"); i >= 0 {
		return m[i+1:]
	}
	return m
}

// semverParts extracts (major, minor) from a "vX.Y.Z" string (pseudo-versions
// and suffixes tolerated). Returns 0,0 on parse failure.
func semverParts(v string) (int, int) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	maj, min := 0, 0
	if len(parts) >= 1 {
		maj, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		min, _ = strconv.Atoi(parts[1])
	}
	return maj, min
}

func semverAtLeast(v string, maj, min int) bool {
	vMaj, vMin := semverParts(v)
	return vMaj > maj || (vMaj == maj && vMin >= min)
}

func semverGreater(a, b string) bool {
	aMaj, aMin := semverParts(a)
	bMaj, bMin := semverParts(b)
	return aMaj > bMaj || (aMaj == bMaj && aMin > bMin)
}
