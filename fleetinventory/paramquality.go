package fleetinventory

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

// Param-level input-schema quality (Anthropic "writing tools for agents":
// every parameter should carry its own description; use enums where a closed
// value set exists). Scored only over tools whose parameters are statically
// visible — today the mcp-go WithString/WithNumber/… inline pattern
// (surfaceinventory.Surface.Params). Struct-based inputs (TypedHandler /
// official-SDK generics) need cross-file type resolution and contribute no
// params, so a repo built entirely on those has no measurable param quality
// (dimension stays nil) rather than being falsely scored — documented as a
// coverage bias, deferred to a future type-resolving extractor.

// enumCandidateName matches parameter names that usually denote a closed value
// set (a good enum candidate). A param so-named without an enum is a soft ding.
var enumCandidateName = regexp.MustCompile(`(?i)(^|_)(type|status|mode|format|kind|level|state|direction|order|sort|severity|method|strategy)s?$`)

// paramQualityScore returns 0-100 over statically-visible params:
//   - primary: fraction of params carrying their own description
//   - secondary: light penalty for enum-candidate params (name looks like a
//     closed set) that declare no enum
//
// nil when no tool in the repo exposed extractable params.
func paramQualityScore(surfaces []surfaceinventory.Surface) (*int, string) {
	total, described, enumCandidates, enumMissing := 0, 0, 0, 0
	toolsWithParams := 0
	for _, s := range surfaces {
		if s.Kind != surfaceinventory.KindMCPTool || len(s.Params) == 0 {
			continue
		}
		toolsWithParams++
		for _, p := range s.Params {
			total++
			if p.HasDescription {
				described++
			}
			if enumCandidateName.MatchString(p.Name) {
				enumCandidates++
				if !p.HasEnum {
					enumMissing++
				}
			}
		}
	}
	if total == 0 {
		return nil, ""
	}
	descShare := float64(described) / float64(total)
	// enum penalty: up to 15 points, proportional to the share of enum-candidate
	// params that lack an enum (0 candidates => no penalty).
	enumPenalty := 0.0
	if enumCandidates > 0 {
		enumPenalty = 15 * float64(enumMissing) / float64(enumCandidates)
	}
	v := clamp(int(100*descShare - enumPenalty + 0.5))
	note := fmt.Sprintf("param quality: %d params across %d tools, %d%% described",
		total, toolsWithParams, int(100*descShare+0.5))
	if enumMissing > 0 {
		note += fmt.Sprintf(", %d enum-candidate param(s) without enum", enumMissing)
	}
	return &v, strings.TrimSpace(note)
}
