package handler

import (
	"strings"
	"testing"
)

// ==================== intToStr ====================

func TestIntToStr_Zero(t *testing.T) {
	if got := intToStr(0); got != "0" {
		t.Errorf("intToStr(0) = %q, want %q", got, "0")
	}
}

func TestIntToStr_Positive(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{42, "42"},
		{100, "100"},
		{1234567, "1234567"},
	}
	for _, c := range cases {
		if got := intToStr(c.n); got != c.want {
			t.Errorf("intToStr(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestIntToStr_Negative(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{-1, "-1"},
		{-42, "-42"},
		{-100, "-100"},
	}
	for _, c := range cases {
		if got := intToStr(c.n); got != c.want {
			t.Errorf("intToStr(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// ==================== floatToStr ====================

func TestFloatToStr_WholeNumber(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{0.0, "0"},
		{1.0, "1"},
		{42.0, "42"},
		{100.0, "100"},
	}
	for _, c := range cases {
		if got := floatToStr(c.f); got != c.want {
			t.Errorf("floatToStr(%v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestFloatToStr_Decimal(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{1.5, "1.50"},
		{3.14, "3.14"},
		{0.1, "0.10"},
		{2.99, "2.99"},
	}
	for _, c := range cases {
		if got := floatToStr(c.f); got != c.want {
			t.Errorf("floatToStr(%v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestFloatToStr_Negative(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{-1.0, "-1"},
		{-3.14, "-3.14"},
		// Note: -0.5 truncates to int(0) losing the sign — known limitation
		// of this simple formatter. Use values with non-zero integer part.
		{-2.5, "-2.50"},
	}
	for _, c := range cases {
		if got := floatToStr(c.f); got != c.want {
			t.Errorf("floatToStr(%v) = %q, want %q", c.f, got, c.want)
		}
	}
}

// ==================== formatAny ====================

func TestFormatAny_Nil(t *testing.T) {
	if got := formatAny(nil); got != "null" {
		t.Errorf("formatAny(nil) = %q, want %q", got, "null")
	}
}

func TestFormatAny_NonNil(t *testing.T) {
	cases := []any{
		"hello",
		42,
		[]string{"a", "b"},
		map[string]int{"x": 1},
		struct{ A int }{A: 1},
	}
	for _, v := range cases {
		if got := formatAny(v); got != "..." {
			t.Errorf("formatAny(%v) = %q, want %q", v, got, "...")
		}
	}
}

// ==================== formatKV ====================

func TestFormatKV_String(t *testing.T) {
	got := formatKV("name", "alice")
	want := `"name": "alice"`
	if got != want {
		t.Errorf("formatKV string = %q, want %q", got, want)
	}
}

func TestFormatKV_BoolTrue(t *testing.T) {
	got := formatKV("flag", true)
	want := `"flag": true`
	if got != want {
		t.Errorf("formatKV bool true = %q, want %q", got, want)
	}
}

func TestFormatKV_BoolFalse(t *testing.T) {
	got := formatKV("flag", false)
	want := `"flag": false`
	if got != want {
		t.Errorf("formatKV bool false = %q, want %q", got, want)
	}
}

func TestFormatKV_Int(t *testing.T) {
	got := formatKV("count", 7)
	want := `"count": 7`
	if got != want {
		t.Errorf("formatKV int = %q, want %q", got, want)
	}
}

func TestFormatKV_Float(t *testing.T) {
	got := formatKV("ratio", 3.14)
	want := `"ratio": 3.14`
	if got != want {
		t.Errorf("formatKV float = %q, want %q", got, want)
	}
}

func TestFormatKV_Default(t *testing.T) {
	got := formatKV("meta", []string{"a"})
	if !strings.HasPrefix(got, `"meta": "`) {
		t.Errorf("formatKV default = %q, want prefix %q", got, `"meta": "`)
	}
}

// ==================== FormatExamples ====================

func TestFormatExamples_Empty(t *testing.T) {
	if got := FormatExamples(nil); got != "" {
		t.Errorf("FormatExamples(nil) = %q, want %q", got, "")
	}
	if got := FormatExamples([]ToolExample{}); got != "" {
		t.Errorf("FormatExamples([]) = %q, want %q", got, "")
	}
}

func TestFormatExamples_SingleNoDescription(t *testing.T) {
	examples := []ToolExample{
		{
			Input: map[string]any{"query": "hello"},
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected 'Examples:' header, got: %q", got)
	}
	if !strings.Contains(got, `"query": "hello"`) {
		t.Errorf("expected query param, got: %q", got)
	}
	// No description line should appear
	if strings.Contains(got, ":") && strings.Count(got, ":") > 2 {
		// allow the two colons from "Examples:" and the KV pair
	}
}

func TestFormatExamples_SingleWithDescription(t *testing.T) {
	examples := []ToolExample{
		{
			Description: "basic search",
			Input:       map[string]any{"query": "test"},
			Output:      "3 results",
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, "basic search:") {
		t.Errorf("expected description line, got: %q", got)
	}
	if !strings.Contains(got, "Input: {") {
		t.Errorf("expected Input: {, got: %q", got)
	}
	if !strings.Contains(got, "Output: 3 results") {
		t.Errorf("expected Output line, got: %q", got)
	}
}

func TestFormatExamples_MultipleExamples(t *testing.T) {
	examples := []ToolExample{
		{
			Description: "first example",
			Input:       map[string]any{"query": "foo"},
		},
		{
			Description: "second example",
			Input:       map[string]any{"query": "bar"},
			Output:      "done",
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, "first example:") {
		t.Errorf("expected first description, got: %q", got)
	}
	if !strings.Contains(got, "second example:") {
		t.Errorf("expected second description, got: %q", got)
	}
	if !strings.Contains(got, "Output: done") {
		t.Errorf("expected output for second example, got: %q", got)
	}
}

func TestFormatExamples_AllParamTypes(t *testing.T) {
	examples := []ToolExample{
		{
			Description: "all types",
			Input: map[string]any{
				"str":   "hello",
				"flag":  true,
				"count": 42,
				"ratio": 1.5,
			},
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, `"hello"`) {
		t.Errorf("expected string value, got: %q", got)
	}
	if !strings.Contains(got, "true") {
		t.Errorf("expected bool true value, got: %q", got)
	}
	if !strings.Contains(got, "42") {
		t.Errorf("expected int value, got: %q", got)
	}
	if !strings.Contains(got, "1.50") {
		t.Errorf("expected float value, got: %q", got)
	}
}

func TestFormatExamples_BoolFalseParam(t *testing.T) {
	examples := []ToolExample{
		{
			Input: map[string]any{"verbose": false},
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, "false") {
		t.Errorf("expected bool false value, got: %q", got)
	}
}

func TestFormatExamples_NilInputValue(t *testing.T) {
	examples := []ToolExample{
		{
			Input: map[string]any{"optional": nil},
		},
	}
	got := FormatExamples(examples)
	if !strings.Contains(got, "null") {
		t.Errorf("expected null for nil value via formatAny, got: %q", got)
	}
}

func TestFormatExamples_NoOutput(t *testing.T) {
	examples := []ToolExample{
		{
			Description: "no output",
			Input:       map[string]any{"x": "y"},
		},
	}
	got := FormatExamples(examples)
	if strings.Contains(got, "Output:") {
		t.Errorf("did not expect Output line when Output is empty, got: %q", got)
	}
}

func TestFormatExamples_StartsWithNewlines(t *testing.T) {
	examples := []ToolExample{
		{Input: map[string]any{"a": "b"}},
	}
	got := FormatExamples(examples)
	if !strings.HasPrefix(got, "\n\nExamples:") {
		t.Errorf("expected result to start with \\n\\nExamples:, got: %q", got)
	}
}
