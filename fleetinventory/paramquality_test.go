package fleetinventory

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

func TestParamQualityNilWhenNoParams(t *testing.T) {
	// TypedHandler-style tools expose no static params -> nil
	surfaces := []surfaceinventory.Surface{
		{Kind: surfaceinventory.KindMCPTool, Name: "t", Pattern: "mcpkit.TypedHandler"},
	}
	if pq, _ := paramQualityScore(surfaces); pq != nil {
		t.Errorf("no-params repo should be nil, got %d", *pq)
	}
}

func TestParamQualityFullyDescribed(t *testing.T) {
	surfaces := []surfaceinventory.Surface{{
		Kind: surfaceinventory.KindMCPTool, Name: "t",
		Params: []surfaceinventory.ToolParam{
			{Name: "query", HasDescription: true, Required: true},
			{Name: "limit", HasDescription: true},
		},
	}}
	pq, _ := paramQualityScore(surfaces)
	if pq == nil || *pq != 100 {
		t.Errorf("fully-described params should be 100, got %v", pq)
	}
}

func TestParamQualityUndescribedAndEnum(t *testing.T) {
	surfaces := []surfaceinventory.Surface{{
		Kind: surfaceinventory.KindMCPTool, Name: "t",
		Params: []surfaceinventory.ToolParam{
			{Name: "query", HasDescription: true},
			{Name: "undoc"},                        // no description
			{Name: "mode"},                         // enum-candidate, no enum, no desc
			{Name: "status", HasDescription: true}, // enum-candidate, no enum
		},
	}}
	pq, note := paramQualityScore(surfaces)
	if pq == nil {
		t.Fatal("nil score")
	}
	// 2/4 described = 50; enum penalty for mode+status (2 candidates, 2 missing) = 15
	if *pq >= 50 {
		t.Errorf("score should reflect 50%% desc minus enum penalty, got %d", *pq)
	}
	if note == "" {
		t.Error("expected a note")
	}
}
