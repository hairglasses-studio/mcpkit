package fleetinventory

import (
	"fmt"
	"math"
	"strings"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

// Tool-count-aware discoverability. Anthropic's tool-search guidance: benefit
// starts at ~10+ tools, and tool-selection accuracy degrades once a server
// exceeds ~30-50 tools without tool-search / deferred loading. For servers at
// that scale, whether tools are grouped into coherent name families (so a
// single BM25/regex tool-search query surfaces a whole family) is the dominant
// practical signal — a large server with a long tail of singleton, ungrouped
// tool names is far harder to work with than an equally-large well-prefixed
// one. Below the threshold, tool-search is unnecessary and namespace coherence
// is a non-issue, so the dimension is not applicable (nil).
//
// This is the static half. Whether a server actually sets deferred loading
// (`defer_loading` / `anthropic/alwaysLoad`) is a config/live signal not
// reliably visible in registration source — deferred to the live variant.
const discoverabilityThreshold = 25

// toolPrefix returns the domain prefix (before the first underscore), or the
// whole name when there is no underscore.
func toolPrefix(name string) string {
	if i := strings.Index(name, "_"); i > 0 {
		return name[:i]
	}
	return name
}

// discoverabilityScore rewards large servers whose tools cluster into coherent
// name families and penalizes ungrouped singleton names. Returns nil (+empty
// note) for servers below the threshold. The note reports the top namespace
// and ungrouped count for the scoreboard.
func discoverabilityScore(surfaces []surfaceinventory.Surface) (*int, string) {
	var names []string
	for _, s := range surfaces {
		if s.Kind == surfaceinventory.KindMCPTool && !isScaffoldName(s.Name) {
			names = append(names, s.Name)
		}
	}
	n := len(names)
	if n < discoverabilityThreshold {
		return nil, ""
	}
	prefixes := map[string]int{}
	for _, nm := range names {
		prefixes[toolPrefix(nm)]++
	}
	inFamily, topCount, top := 0, 0, ""
	for p, c := range prefixes {
		if c >= 2 { // a name family (findable as a group)
			inFamily += c
		}
		if c > topCount {
			topCount, top = c, p
		}
	}
	coherence := float64(inFamily) / float64(n) // share of tools in a family
	dominance := float64(topCount) / float64(n) // largest single namespace share
	domTerm := math.Min(dominance/0.5, 1)       // a 50%+ dominant namespace = full marks on that term
	score := clamp(int(math.Round(100 * (0.7*coherence + 0.3*domTerm))))
	ungrouped := n - inFamily
	note := fmt.Sprintf("discoverability: %d tools, top prefix %q %.0f%%, %d ungrouped", n, top, dominance*100, ungrouped)
	return &score, note
}
