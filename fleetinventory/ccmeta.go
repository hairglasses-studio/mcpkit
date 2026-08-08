package fleetinventory

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Claude Code _meta tool-extension adoption. Claude Code recognizes three
// per-tool _meta keys (code.claude.com/docs/en/mcp) that are NOT part of the
// generic MCP spec — generic MCP linters miss them:
//   - anthropic/alwaysLoad          — opt a tool OUT of deferred/tool-search
//                                      loading (per-tool "always in context")
//   - anthropic/maxResultSizeChars  — raise the persist-to-disk output ceiling
//                                      for that tool (hard max 500,000)
//   - anthropic/requiresUserInteraction — force a permission prompt on every
//                                      call (ignores allow rules / bypass mode)
//
// In mcpkit these are set via registry.ToolDefinition fields (AlwaysLoad /
// MaxResultChars / SkipRequiresUserInteraction) and injected into tools/list
// _meta by ApplyToolMetadata. This is an INFORMATIONAL adoption signal, not a
// scored dimension. Key conformance point: Claude Code enables tool-search
// (deferred loading) BY DEFAULT, so anthropic/alwaysLoad is an OPT-OUT for the
// few hottest tools per server — setting it broadly DEFEATS deferred loading
// and is flagged.
//
// Detection is USE-pattern only (a struct-field assignment or literal key), so
// the mcpkit struct DEFINITION (`AlwaysLoad bool`) and read-sites
// (`if td.AlwaysLoad`) do not false-match.

// ccMetaScanBytes bounds the per-file read. These setters cluster near tool
// registration, so a moderate header read catches adoption without the ~2x
// full-fleet scan-time cost of reading every .go body in full. Counts may
// under-count in very large module files — acceptable for an adoption signal
// (the conclusion is "does the fleet use these", not an exact tally).
const ccMetaScanBytes = 12288

// alwaysLoadOveruseThreshold: more tools than this opting out of deferred
// loading on one server undermines tool-search — worth flagging.
const alwaysLoadOveruseThreshold = 8

var (
	ccAlwaysLoadRe  = regexp.MustCompile(`AlwaysLoad:\s*true|\.AlwaysLoad\s*=\s*true`)
	ccMaxResultRe   = regexp.MustCompile(`MaxResultChars:\s*[1-9]|\.MaxResultChars\s*=\s*[1-9]`)
	ccRequiresRe    = regexp.MustCompile(`SkipRequiresUserInteraction:\s*true|\.SkipRequiresUserInteraction\s*=\s*true`)
	ccLiteralMetaRe = regexp.MustCompile(`anthropic/(alwaysLoad|maxResultSizeChars|requiresUserInteraction)`)
)

// CCMetaProfile is a repo's Claude Code _meta extension adoption.
type CCMetaProfile struct {
	AlwaysLoad       int    `json:"always_load,omitempty"`
	MaxResultChars   int    `json:"max_result_chars,omitempty"`
	SkipRequiresUser int    `json:"skip_requires_user_interaction,omitempty"`
	LiteralMetaKeys  int    `json:"literal_meta_keys,omitempty"`
	Warning          string `json:"warning,omitempty"`
}

// Any reports whether the repo uses any Claude Code _meta extension.
func (c CCMetaProfile) Any() bool {
	return c.AlwaysLoad+c.MaxResultChars+c.SkipRequiresUser+c.LiteralMetaKeys > 0
}

// detectCCMeta counts Claude Code _meta extension usage across a repo's .go
// source (bounded read per file).
func detectCCMeta(dir string, files []string) CCMetaProfile {
	var p CCMetaProfile
	buf := make([]byte, ccMetaScanBytes)
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		// Skip this detector's own source — it embeds the patterns as literals.
		if strings.HasSuffix(rel, "fleetinventory/ccmeta.go") || rel == "ccmeta.go" {
			continue
		}
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		n, _ := f.Read(buf)
		f.Close()
		s := string(buf[:n])
		// Cheap substring gate — the vast majority of .go files contain none
		// of these tokens, so skip the four regexes entirely for them.
		if !strings.Contains(s, "AlwaysLoad") && !strings.Contains(s, "MaxResultChars") &&
			!strings.Contains(s, "anthropic/") && !strings.Contains(s, "SkipRequiresUserInteraction") {
			continue
		}
		p.AlwaysLoad += len(ccAlwaysLoadRe.FindAllString(s, -1))
		p.MaxResultChars += len(ccMaxResultRe.FindAllString(s, -1))
		p.SkipRequiresUser += len(ccRequiresRe.FindAllString(s, -1))
		p.LiteralMetaKeys += len(ccLiteralMetaRe.FindAllString(s, -1))
	}
	if p.AlwaysLoad > alwaysLoadOveruseThreshold {
		p.Warning = "anthropic/alwaysLoad set on many tools — opting a large share out of deferred loading undermines tool-search (keep it to the few hottest tools)"
	}
	return p
}
