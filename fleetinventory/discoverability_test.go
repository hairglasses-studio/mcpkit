package fleetinventory

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

func mkTools(names ...string) []surfaceinventory.Surface {
	out := make([]surfaceinventory.Surface, 0, len(names))
	for _, n := range names {
		out = append(out, surfaceinventory.Surface{Kind: surfaceinventory.KindMCPTool, Name: n})
	}
	return out
}

func TestDiscoverabilityBelowThreshold(t *testing.T) {
	var names []string
	for i := 0; i < 10; i++ {
		names = append(names, "svc_tool"+string(rune('a'+i)))
	}
	if d, _ := discoverabilityScore(mkTools(names...)); d != nil {
		t.Errorf("below threshold should be nil, got %d", *d)
	}
}

func TestDiscoverabilityWellPrefixed(t *testing.T) {
	// 30 tools all sharing "aftrs_" prefix -> coherent, dominant -> high score
	var names []string
	for i := 0; i < 30; i++ {
		names = append(names, "aftrs_tool"+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	d, _ := discoverabilityScore(mkTools(names...))
	if d == nil || *d < 90 {
		t.Errorf("well-prefixed large server should score high, got %v", d)
	}
}

func TestDiscoverabilityFragmented(t *testing.T) {
	// 30 tools each with a unique singleton prefix -> no families -> low score
	var names []string
	for i := 0; i < 30; i++ {
		names = append(names, "uniq"+string(rune('a'+i%26))+string(rune('0'+i/26))+"_do")
	}
	d, note := discoverabilityScore(mkTools(names...))
	if d == nil || *d > 40 {
		t.Errorf("fragmented singleton-prefix server should score low, got %v", d)
	}
	if note == "" {
		t.Error("expected a discoverability note")
	}
}

func TestDiscoverabilityMultipleFamilies(t *testing.T) {
	// two coherent families (arr_*, jelly_*) covering all tools -> high coherence
	// even without one dominant namespace.
	var names []string
	for i := 0; i < 15; i++ {
		names = append(names, "arr_x"+string(rune('a'+i)))
	}
	for i := 0; i < 15; i++ {
		names = append(names, "jelly_y"+string(rune('a'+i)))
	}
	d, _ := discoverabilityScore(mkTools(names...))
	if d == nil || *d < 75 {
		t.Errorf("two coherent families should score well, got %v", d)
	}
}
