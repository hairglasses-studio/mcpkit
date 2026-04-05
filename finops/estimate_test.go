//go:build !official_sdk

package finops

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TestDefaultEstimate_Empty verifies that an empty string returns 0.
func TestDefaultEstimate_Empty(t *testing.T) {
	t.Parallel()

	if got := DefaultEstimate(""); got != 0 {
		t.Errorf("DefaultEstimate(%q): expected 0, got %d", "", got)
	}
}

// TestDefaultEstimate_Short covers short strings where the result is determined
// by ceiling division of a small character count.
func TestDefaultEstimate_Short(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  int
	}{
		{"a", 1},     // 1 char  → ceil(1/4) = 1
		{"ab", 1},    // 2 chars → ceil(2/4) = 1
		{"abc", 1},   // 3 chars → ceil(3/4) = 1
		{"abcd", 1},  // 4 chars → ceil(4/4) = 1
		{"abcde", 2}, // 5 chars → ceil(5/4) = 2
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := DefaultEstimate(tc.input)
			if got != tc.want {
				t.Errorf("DefaultEstimate(%q): expected %d, got %d", tc.input, tc.want, got)
			}
		})
	}
}

// TestDefaultEstimate_Long verifies ceiling division for longer strings and
// confirms the formula (len+3)/4.
func TestDefaultEstimate_Long(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  int
	}{
		{"12345678", 2},          // 8 chars  → (8+3)/4 = 2
		{"123456789", 3},         // 9 chars  → (9+3)/4 = 3
		{"1234567890123456", 4},  // 16 chars → (16+3)/4 = 4
		{"12345678901234567", 5}, // 17 chars → (17+3)/4 = 5
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := DefaultEstimate(tc.input)
			if got != tc.want {
				t.Errorf("DefaultEstimate(%q): expected %d, got %d", tc.input, tc.want, got)
			}
		})
	}
}

// TestDefaultEstimate_CeilingProperty verifies the ceiling property: result must
// be exactly ceil(len/4) which equals (len+3)/4 in integer arithmetic.
func TestDefaultEstimate_CeilingProperty(t *testing.T) {
	t.Parallel()

	// Build strings of length 0–20 and verify formula holds for each.
	for n := 0; n <= 20; n++ {
		input := make([]byte, n)
		for i := range input {
			input[i] = 'x'
		}
		text := string(input)
		want := 0
		if n > 0 {
			want = (n + 3) / 4
		}
		got := DefaultEstimate(text)
		if got != want {
			t.Errorf("len=%d: expected %d, got %d", n, want, got)
		}
	}
}

// TestEstimateFromRequest_NilArgs verifies that a request with nil arguments
// returns 0 (no tokens to count).
func TestEstimateFromRequest_NilArgs(t *testing.T) {
	t.Parallel()

	req := makeTestRequest("noop", nil)
	got := EstimateFromRequest(req, DefaultEstimate)
	if got != 0 {
		t.Errorf("EstimateFromRequest with nil args: expected 0, got %d", got)
	}
}

// TestEstimateFromRequest_WithArgs verifies that a request with non-trivial
// arguments produces a positive estimate.
func TestEstimateFromRequest_WithArgs(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"query":  "what is the meaning of life",
		"limit":  42,
		"active": true,
	}
	req := makeTestRequest("search", args)
	got := EstimateFromRequest(req, DefaultEstimate)
	if got <= 0 {
		t.Errorf("EstimateFromRequest with args: expected positive estimate, got %d", got)
	}
}

// TestEstimateFromRequest_CustomEstimator verifies that EstimateFromRequest
// delegates to the provided estimator function.
func TestEstimateFromRequest_CustomEstimator(t *testing.T) {
	t.Parallel()

	// Custom estimator always returns a fixed sentinel value per character.
	const tokensPerChar = 7
	custom := func(text string) int { return len(text) * tokensPerChar }

	args := map[string]any{"key": "val"}
	req := makeTestRequest("tool", args)

	got := EstimateFromRequest(req, custom)
	// The JSON of {"key":"val"} is 13 chars → 13*7 = 91, but exact JSON may vary;
	// just assert the result uses the custom multiplier (will be > DefaultEstimate).
	def := EstimateFromRequest(req, DefaultEstimate)
	if got <= def {
		t.Errorf("expected custom estimator result (%d) > default (%d)", got, def)
	}
}

// TestEstimateFromResult_NilResult verifies that a nil result returns 0.
func TestEstimateFromResult_NilResult(t *testing.T) {
	t.Parallel()

	got := EstimateFromResult(nil, DefaultEstimate)
	if got != 0 {
		t.Errorf("EstimateFromResult(nil): expected 0, got %d", got)
	}
}

// TestEstimateFromResult_EmptyContent verifies that a result with no content
// items returns 0.
func TestEstimateFromResult_EmptyContent(t *testing.T) {
	t.Parallel()

	result := &registry.CallToolResult{
		Content: nil,
	}
	got := EstimateFromResult(result, DefaultEstimate)
	if got != 0 {
		t.Errorf("EstimateFromResult with empty content: expected 0, got %d", got)
	}
}

// TestEstimateFromResult_SingleTextContent verifies estimation for a result
// with a single text content item.
func TestEstimateFromResult_SingleTextContent(t *testing.T) {
	t.Parallel()

	text := "the quick brown fox"
	result := registry.MakeTextResult(text)
	want := DefaultEstimate(text)

	got := EstimateFromResult(result, DefaultEstimate)
	if got != want {
		t.Errorf("EstimateFromResult single content: expected %d, got %d", want, got)
	}
}

// TestEstimateFromResult_MultiContent verifies that EstimateFromResult sums
// token estimates across all content items in the result.
func TestEstimateFromResult_MultiContent(t *testing.T) {
	t.Parallel()

	text1 := "first content item"
	text2 := "second content item longer"
	text3 := "third"

	result := &registry.CallToolResult{
		Content: []registry.Content{
			registry.MakeTextContent(text1),
			registry.MakeTextContent(text2),
			registry.MakeTextContent(text3),
		},
	}

	want := DefaultEstimate(text1) + DefaultEstimate(text2) + DefaultEstimate(text3)
	got := EstimateFromResult(result, DefaultEstimate)
	if got != want {
		t.Errorf("EstimateFromResult multi-content: expected %d, got %d", want, got)
	}
}

// TestEstimateFromResult_MultiContent_CustomEstimator verifies that a custom
// estimator is applied to each content item independently.
func TestEstimateFromResult_MultiContent_CustomEstimator(t *testing.T) {
	t.Parallel()

	// Always-1 estimator: every non-empty text costs exactly 1 token.
	always1 := func(text string) int {
		if len(text) == 0 {
			return 0
		}
		return 1
	}

	result := &registry.CallToolResult{
		Content: []registry.Content{
			registry.MakeTextContent("aaa"),
			registry.MakeTextContent("bbb"),
			registry.MakeTextContent("ccc"),
		},
	}

	got := EstimateFromResult(result, always1)
	if got != 3 {
		t.Errorf("expected 3 (one token per content item), got %d", got)
	}
}
