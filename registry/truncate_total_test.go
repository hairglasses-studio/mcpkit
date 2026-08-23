package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// truncationMarkerAllowance is the slack this file's total-size assertions
// grant for the truncation marker that truncateResponse appends to the text
// it cuts. It is deliberately a small fixed number: the point of the guard is
// that the total lands *at* the cap plus a notice, not that it lands within
// some percentage of it. Do not grow this to make a test pass — growing it is
// the same defect as raising the cap.
const truncationMarkerAllowance = 512

// resultSizes reports the serialized size of each half of a tool result:
// every text content block, and the structuredContent object. It also reports
// whether any text block carries the truncation marker.
//
// This mirrors how a caller actually pays for a result — Claude Code stores
// the compact structuredContent in the transcript when it is present and
// falls back to the text block when it is not, so a cap that clips only one
// half is a cap on nothing.
func resultSizes(t *testing.T, result *CallToolResult) (textBytes, structBytes int, marker bool) {
	t.Helper()
	if result == nil {
		return 0, 0, false
	}
	for _, c := range result.Content {
		if txt, ok := ExtractTextContent(c); ok {
			textBytes += len(txt)
			if strings.Contains(txt, "[TRUNCATED:") {
				marker = true
			}
		}
	}
	if result.StructuredContent != nil {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structuredContent: %v", err)
		}
		structBytes = len(encoded)
	}
	return textBytes, structBytes, marker
}

// bigStructuredPayload builds a payload whose JSON serialization comfortably
// exceeds any small test cap, shaped exactly like handler.StructuredResult's
// output: an indented text rendering plus the SAME data as structuredContent.
func bigStructuredPayload(rows int) (data any, indented string) {
	type row struct {
		Name string `json:"name"`
		Blob string `json:"blob"`
	}
	payload := struct {
		Rows  []row `json:"rows"`
		Total int   `json:"total"`
	}{}
	for i := range rows {
		payload.Rows = append(payload.Rows, row{
			Name: fmt.Sprintf("row-%04d", i),
			Blob: strings.Repeat("x", 64),
		})
	}
	payload.Total = len(payload.Rows)
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(err)
	}
	return payload, string(encoded)
}

// TestRegistryTruncationIsTotalNotPerField is the regression guard for the
// 2026-08-23 result-size defect (secretstudios-mcp
// notes/tool-result-size-audit-2026-08-23.md §4.2).
//
// The defect: truncateResponse enforced the cap on text content blocks ONLY.
// A result built by handler.StructuredResult carries the same data twice —
// indented in content[0].text and again in structuredContent — so the guard
// clipped the text half, appended "[TRUNCATED: response exceeded NKB limit]"
// to it, and shipped the complete structuredContent alongside it untouched.
// The caller was told the response was truncated while the full payload was
// right there in the same result, and the server shipped ~2.7x its own cap.
// Measured live on secretstudios_tool_catalog: 353,127 wire bytes against a
// 131,072-byte cap, text clipped at the cap with the marker, structuredContent
// carrying all 369 tools complete.
//
// The invariant this test encodes is deliberately implementation-agnostic:
// the two representations must not contradict each other, and the TOTAL must
// respect the cap. It does not care which half is dropped.
func TestRegistryTruncationIsTotalNotPerField(t *testing.T) {
	const maxSize = 4096

	r := NewToolRegistry(Config{MaxResponseSize: maxSize})
	data, indented := bigStructuredPayload(200)

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("big_structured_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				return MakeStructuredResult(MakeTextContent(indented), data), nil
			}),
		},
	})

	td, ok := r.GetTool("big_structured_tool")
	if !ok {
		t.Fatal("big_structured_tool not registered")
	}
	result, err := r.wrapHandler("big_structured_tool", td)(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textBytes, structBytes, marker := resultSizes(t, result)

	// Precondition, asserted rather than assumed: this fixture must actually
	// be over budget, or the rest of the test proves nothing. A vacuous pass
	// here would be the exact defect class this guard exists to prevent.
	if !marker {
		t.Fatalf("fixture is not over budget: no truncation marker in %d bytes of text (cap %d) — the guard cannot fire", textBytes, maxSize)
	}

	if structBytes > 0 {
		t.Errorf("truncation marker present but structuredContent still carries %d bytes: the cap is enforced on the text half and bypassed on the structured half, and the marker contradicts the payload", structBytes)
	}
	if total := textBytes + structBytes; total > maxSize+truncationMarkerAllowance {
		t.Errorf("cap bypassed: result total %d bytes (text %d + structuredContent %d) exceeds max %d (+%d marker allowance)",
			total, textBytes, structBytes, maxSize, truncationMarkerAllowance)
	}
}

// TestRegistryTruncationBudgetIsSharedAcrossContentBlocks covers the second
// half of the same per-field-instead-of-total defect: the cap was applied to
// each text block independently, so N blocks of maxSize-1 bytes each shipped
// N*(maxSize-1) bytes with no marker anywhere.
func TestRegistryTruncationBudgetIsSharedAcrossContentBlocks(t *testing.T) {
	const maxSize = 1024

	r := NewToolRegistry(Config{MaxResponseSize: maxSize})
	block := strings.Repeat("y", maxSize-1)

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("multiblock_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				return &CallToolResult{Content: []Content{
					MakeTextContent(block),
					MakeTextContent(block),
					MakeTextContent(block),
				}}, nil
			}),
		},
	})

	td, ok := r.GetTool("multiblock_tool")
	if !ok {
		t.Fatal("multiblock_tool not registered")
	}
	result, err := r.wrapHandler("multiblock_tool", td)(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textBytes, structBytes, marker := resultSizes(t, result)
	if total := textBytes + structBytes; total > maxSize+truncationMarkerAllowance {
		t.Errorf("per-block cap instead of total: result total %d bytes across %d content blocks exceeds max %d (+%d marker allowance)",
			total, len(result.Content), maxSize, truncationMarkerAllowance)
	}
	if !marker {
		t.Error("an over-budget multi-block result carries no truncation marker: the caller is told nothing was dropped")
	}
}

// TestRegistryTruncationLeavesUnderBudgetResultsAlone pins the other side of
// the contract, so a future "just drop structuredContent everywhere" change
// cannot pass this file. Under-budget results keep BOTH representations
// byte-for-byte.
func TestRegistryTruncationLeavesUnderBudgetResultsAlone(t *testing.T) {
	r := NewToolRegistry(Config{MaxResponseSize: 128 * 1024})
	data, indented := bigStructuredPayload(3)

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("small_structured_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				return MakeStructuredResult(MakeTextContent(indented), data), nil
			}),
		},
	})

	td, ok := r.GetTool("small_structured_tool")
	if !ok {
		t.Fatal("small_structured_tool not registered")
	}
	result, err := r.wrapHandler("small_structured_tool", td)(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textBytes, structBytes, marker := resultSizes(t, result)
	if marker {
		t.Error("under-budget result was marked truncated")
	}
	if structBytes == 0 {
		t.Error("under-budget result lost its structuredContent")
	}
	if textBytes != len(indented) {
		t.Errorf("under-budget text changed: got %d bytes, want %d", textBytes, len(indented))
	}
}
