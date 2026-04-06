package session

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// makeTestThread creates a thread with N events of mixed types for testing.
func makeTestThread(n int) *Thread {
	th := NewThreadWithID("ctx-test")
	base := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)

	types := []EventType{
		EventToolCall,
		EventToolResult,
		EventError,
		EventCheckpoint,
		EventSystemMessage,
	}

	for i := range n {
		th.Append(Event{
			ID:        "e-" + itoa(i),
			Type:      types[i%len(types)],
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Data:      "data-" + itoa(i),
			Metadata:  map[string]string{"index": itoa(i)},
		})
	}
	return th
}

func TestContextBuilder_JSON(t *testing.T) {
	th := makeTestThread(3)
	cb := NewContextBuilder(ContextFormatJSON, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build JSON: %v", err)
	}

	// Should be valid JSON.
	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("result is not valid JSON: %v\nresult: %s", err, result)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify types round-trip.
	if events[0].Type != EventToolCall {
		t.Errorf("event[0] type: got %q, want %q", events[0].Type, EventToolCall)
	}
	if events[1].Type != EventToolResult {
		t.Errorf("event[1] type: got %q, want %q", events[1].Type, EventToolResult)
	}
	if events[2].Type != EventError {
		t.Errorf("event[2] type: got %q, want %q", events[2].Type, EventError)
	}

	// Verify timestamps are RFC3339.
	for i, e := range events {
		if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
			t.Errorf("event[%d] timestamp not RFC3339: %q", i, e.Timestamp)
		}
	}

	// Verify metadata present.
	if events[0].Metadata["index"] != "0" {
		t.Errorf("event[0] metadata index: got %q, want %q", events[0].Metadata["index"], "0")
	}
}

func TestContextBuilder_JSON_Empty(t *testing.T) {
	th := NewThreadWithID("empty")
	cb := NewContextBuilder(ContextFormatJSON, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build JSON empty: %v", err)
	}

	if result != "[]" {
		t.Fatalf("expected empty JSON array, got: %s", result)
	}
}

func TestContextBuilder_XML(t *testing.T) {
	th := makeTestThread(3)
	cb := NewContextBuilder(ContextFormatXML, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build XML: %v", err)
	}

	// Should have context root element.
	if !strings.HasPrefix(result, "<context>") {
		t.Errorf("expected <context> prefix, got: %s", result[:min(50, len(result))])
	}
	if !strings.HasSuffix(result, "</context>") {
		t.Errorf("expected </context> suffix")
	}

	// Event types become XML tags.
	if !strings.Contains(result, "<tool_call") {
		t.Error("expected <tool_call> tag in XML output")
	}
	if !strings.Contains(result, "</tool_call>") {
		t.Error("expected </tool_call> closing tag")
	}
	if !strings.Contains(result, "<tool_result") {
		t.Error("expected <tool_result> tag in XML output")
	}
	if !strings.Contains(result, "<error") {
		t.Error("expected <error> tag in XML output")
	}

	// Verify data elements.
	if !strings.Contains(result, "<data>data-0</data>") {
		t.Error("expected <data>data-0</data> in XML output")
	}

	// Verify metadata elements.
	if !strings.Contains(result, "<index>") {
		t.Error("expected <index> metadata tag in XML output")
	}
}

func TestContextBuilder_XML_Escaping(t *testing.T) {
	th := NewThreadWithID("xml-escape")
	th.Append(Event{
		ID:        "e1",
		Type:      EventToolResult,
		Timestamp: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
		Data:      `<script>alert("xss")</script>`,
		Metadata:  map[string]string{"key": `a&b<c>d"e`},
	})

	cb := NewContextBuilder(ContextFormatXML, 0)
	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build XML escape: %v", err)
	}

	// Data should be escaped.
	if strings.Contains(result, `<script>`) {
		t.Error("XML data not properly escaped: found raw <script> tag")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Error("expected escaped <script> in XML output")
	}

	// Metadata values should be escaped.
	if !strings.Contains(result, "a&amp;b&lt;c&gt;d&quot;e") {
		t.Errorf("expected escaped metadata value in XML output, got:\n%s", result)
	}
}

func TestContextBuilder_YAML(t *testing.T) {
	th := makeTestThread(3)
	cb := NewContextBuilder(ContextFormatYAML, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build YAML: %v", err)
	}

	// Should have YAML list items.
	lines := strings.Split(result, "\n")
	listItems := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- type:") {
			listItems++
		}
	}
	if listItems != 3 {
		t.Fatalf("expected 3 YAML list items, got %d", listItems)
	}

	// Verify structure.
	if !strings.Contains(result, "- type: tool_call") {
		t.Error("expected '- type: tool_call' in YAML output")
	}
	if !strings.Contains(result, "  timestamp:") {
		t.Error("expected '  timestamp:' in YAML output")
	}
	if !strings.Contains(result, "  data:") {
		t.Error("expected '  data:' in YAML output")
	}
	if !strings.Contains(result, "  metadata:") {
		t.Error("expected '  metadata:' in YAML output")
	}
}

func TestContextBuilder_Compact(t *testing.T) {
	th := makeTestThread(3)
	cb := NewContextBuilder(ContextFormatCompact, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build Compact: %v", err)
	}

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Verify compact format: [TYPE@TIME] DATA {k=v}
	if !strings.Contains(lines[0], "[tool_call@10:00:00]") {
		t.Errorf("line 0: expected [tool_call@10:00:00], got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "data-0") {
		t.Errorf("line 0: expected data-0, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "index=0") {
		t.Errorf("line 0: expected index=0 metadata, got: %s", lines[0])
	}

	if !strings.Contains(lines[1], "[tool_result@10:01:00]") {
		t.Errorf("line 1: expected [tool_result@10:01:00], got: %s", lines[1])
	}
}

func TestContextBuilder_Compact_TokenEfficiency(t *testing.T) {
	// Compact format should be significantly smaller than JSON.
	th := makeTestThread(10)

	jsonCB := NewContextBuilder(ContextFormatJSON, 0)
	jsonResult, err := jsonCB.Build(th)
	if err != nil {
		t.Fatalf("Build JSON: %v", err)
	}

	compactCB := NewContextBuilder(ContextFormatCompact, 0)
	compactResult, err := compactCB.Build(th)
	if err != nil {
		t.Fatalf("Build Compact: %v", err)
	}

	jsonTokens := TokenCount(jsonResult)
	compactTokens := TokenCount(compactResult)

	if compactTokens >= jsonTokens {
		t.Errorf("compact (%d tokens) should be smaller than JSON (%d tokens)",
			compactTokens, jsonTokens)
	}
}

func TestContextBuilder_TokenBudget(t *testing.T) {
	th := makeTestThread(20)
	// Use a very small token budget to force truncation.
	cb := NewContextBuilder(ContextFormatCompact, 50)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with budget: %v", err)
	}

	tokens := TokenCount(result)
	if tokens > 50 {
		t.Fatalf("result exceeds token budget: %d tokens > 50 budget", tokens)
	}

	// Should have fewer events than the original 20.
	lines := strings.Split(result, "\n")
	nonEmpty := 0
	for _, l := range lines {
		if l != "" {
			nonEmpty++
		}
	}
	if nonEmpty >= 20 {
		t.Fatalf("expected fewer than 20 events after truncation, got %d", nonEmpty)
	}

	// The surviving events should be the most recent ones (oldest truncated first).
	if len(result) > 0 && !strings.Contains(result, "data-19") {
		t.Error("expected the most recent event (data-19) to survive truncation")
	}
}

func TestContextBuilder_TokenBudget_Zero(t *testing.T) {
	th := makeTestThread(5)
	// Zero budget means no limit.
	cb := NewContextBuilder(ContextFormatJSON, 0)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with zero budget: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected all 5 events with zero budget, got %d", len(events))
	}
}

func TestContextBuilder_TokenBudget_VerySmall(t *testing.T) {
	th := makeTestThread(10)
	// Budget so small that no events can fit.
	cb := NewContextBuilder(ContextFormatJSON, 1)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with tiny budget: %v", err)
	}

	// Should either be empty or the minimal representation.
	if TokenCount(result) > 1 {
		t.Fatalf("expected at most 1 token, got %d", TokenCount(result))
	}
}

func TestContextBuilder_FilterByType(t *testing.T) {
	th := makeTestThread(10) // types cycle: call, result, error, checkpoint, system

	cb := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterByType(EventToolCall, EventToolResult))

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with type filter: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// 10 events cycling through 5 types: call, result, error, checkpoint, system
	// call: indices 0,5; result: indices 1,6 => 4 events total
	if len(events) != 4 {
		t.Fatalf("expected 4 events (call+result), got %d", len(events))
	}

	for _, e := range events {
		if e.Type != EventToolCall && e.Type != EventToolResult {
			t.Errorf("unexpected event type: %q", e.Type)
		}
	}
}

func TestContextBuilder_FilterExcludeType(t *testing.T) {
	th := makeTestThread(10)

	cb := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterExcludeType(EventError, EventCheckpoint))

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with exclude filter: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// 10 events, exclude error (indices 2,7) and checkpoint (indices 3,8) => 6 remaining
	if len(events) != 6 {
		t.Fatalf("expected 6 events after exclusion, got %d", len(events))
	}

	for _, e := range events {
		if e.Type == EventError || e.Type == EventCheckpoint {
			t.Errorf("excluded event type leaked through: %q", e.Type)
		}
	}
}

func TestContextBuilder_FilterAfter(t *testing.T) {
	th := makeTestThread(10)
	// Events are at 10:00, 10:01, ..., 10:09
	cutoff := time.Date(2026, 4, 5, 10, 5, 0, 0, time.UTC)

	cb := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterAfter(cutoff))

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with after filter: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Events at 10:06, 10:07, 10:08, 10:09 => 4 events
	if len(events) != 4 {
		t.Fatalf("expected 4 events after cutoff, got %d", len(events))
	}

	for _, e := range events {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			t.Fatalf("parse timestamp: %v", err)
		}
		if !ts.After(cutoff) {
			t.Errorf("event timestamp %v should be after %v", ts, cutoff)
		}
	}
}

func TestContextBuilder_FilterLastN(t *testing.T) {
	th := makeTestThread(10)

	cb := NewContextBuilder(ContextFormatJSON, 0)
	cb.FilterLastN(3)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with lastN filter: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Should be the last 3: indices 7, 8, 9
	if events[0].Metadata["index"] != "7" {
		t.Errorf("first event index: got %q, want %q", events[0].Metadata["index"], "7")
	}
	if events[2].Metadata["index"] != "9" {
		t.Errorf("last event index: got %q, want %q", events[2].Metadata["index"], "9")
	}
}

func TestContextBuilder_FilterLastN_LargerThanTotal(t *testing.T) {
	th := makeTestThread(3)

	cb := NewContextBuilder(ContextFormatJSON, 0)
	cb.FilterLastN(100)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build with large lastN: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Should keep all 3.
	if len(events) != 3 {
		t.Fatalf("expected 3 events (all), got %d", len(events))
	}
}

func TestContextBuilder_CombinedFilters(t *testing.T) {
	th := makeTestThread(20)
	// Events cycle: call(0,5,10,15), result(1,6,11,16), error(2,7,12,17),
	//               checkpoint(3,8,13,18), system(4,9,14,19)

	cutoff := time.Date(2026, 4, 5, 10, 10, 0, 0, time.UTC) // after index 10

	cb := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterByType(EventToolCall, EventToolResult)).
		WithFilter(FilterAfter(cutoff))
	cb.FilterLastN(2)

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build combined: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// After cutoff (>10:10): indices 11-19
	// Of those, type call or result: 11(result), 15(call), 16(result)
	// LastN(2): 15(call), 16(result)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != EventToolCall {
		t.Errorf("event[0] type: got %q, want %q", events[0].Type, EventToolCall)
	}
	if events[1].Type != EventToolResult {
		t.Errorf("event[1] type: got %q, want %q", events[1].Type, EventToolResult)
	}
}

func TestContextBuilder_CombinedFilters_TypeAndExclude(t *testing.T) {
	th := makeTestThread(10)

	// Include only tool calls, but also exclude tool calls -- should produce nothing.
	cb := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterByType(EventToolCall)).
		WithFilter(FilterExcludeType(EventToolCall))

	result, err := cb.Build(th)
	if err != nil {
		t.Fatalf("Build contradictory filters: %v", err)
	}

	if result != "[]" {
		t.Fatalf("expected empty JSON array, got: %s", result)
	}
}

func TestContextBuilder_NilThread(t *testing.T) {
	cb := NewContextBuilder(ContextFormatJSON, 0)
	_, err := cb.Build(nil)
	if err == nil {
		t.Fatal("expected error for nil thread")
	}
}

func TestContextBuilder_UnsupportedFormat(t *testing.T) {
	th := makeTestThread(1)
	cb := NewContextBuilder("unknown", 0)
	_, err := cb.Build(th)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestTokenCount(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"12345678", 2},
		{"123456789", 3},
		{strings.Repeat("x", 100), 25},
		{strings.Repeat("x", 101), 26},
		{strings.Repeat("x", 400), 100},
	}

	for _, tt := range tests {
		got := TokenCount(tt.input)
		if got != tt.expected {
			t.Errorf("TokenCount(%d chars): got %d, want %d", len(tt.input), got, tt.expected)
		}
	}
}

func TestTokenCount_Consistency(t *testing.T) {
	// Verify that TokenCount is monotonically increasing.
	prev := 0
	for i := range 200 {
		s := strings.Repeat("a", i)
		count := TokenCount(s)
		if count < prev {
			t.Fatalf("TokenCount not monotonic: %d chars -> %d tokens, but %d chars -> %d tokens",
				i-1, prev, i, count)
		}
		prev = count
	}
}

func TestContextBuilder_AllFormats_SameEvents(t *testing.T) {
	th := makeTestThread(5)
	formats := []ContextFormat{
		ContextFormatJSON,
		ContextFormatXML,
		ContextFormatYAML,
		ContextFormatCompact,
	}

	for _, f := range formats {
		cb := NewContextBuilder(f, 0)
		result, err := cb.Build(th)
		if err != nil {
			t.Errorf("format %q: Build error: %v", f, err)
			continue
		}
		if result == "" {
			t.Errorf("format %q: empty result", f)
		}
	}
}

func TestContextBuilder_Chaining(t *testing.T) {
	th := makeTestThread(10)

	// Test that WithFilter returns the builder for method chaining.
	result, err := NewContextBuilder(ContextFormatJSON, 0).
		WithFilter(FilterByType(EventToolCall)).
		Build(th)
	if err != nil {
		t.Fatalf("chained Build: %v", err)
	}

	var events []contextEvent
	if err := json.Unmarshal([]byte(result), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 tool_call events, got %d", len(events))
	}
}

func TestXMLSafeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tool_call", "tool_call"},
		{"tool_result", "tool_result"},
		{"error", "error"},
		{"human_request", "human_request"},
		{"123bad", "_23bad"},    // leading digit becomes _
		{"a b c", "a_b_c"},     // spaces become _
		{"", "_"},              // empty becomes _
		{"valid-tag.1", "valid-tag.1"},
	}

	for _, tt := range tests {
		got := xmlSafeTag(tt.input)
		if got != tt.expected {
			t.Errorf("xmlSafeTag(%q): got %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"<br>", "&lt;br&gt;"},
		{"a&b", "a&amp;b"},
		{`say "hi"`, `say &quot;hi&quot;`},
		{"<a>&b\"c", "&lt;a&gt;&amp;b&quot;c"},
	}

	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.expected {
			t.Errorf("xmlEscape(%q): got %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestYAMLEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"", `""`},
		{"has: colon", `"has: colon"`},
		{"has # comment", `"has # comment"`},
		{"has\nnewline", "\"has\nnewline\""},
		{`has "quote"`, `"has \"quote\""`},
	}

	for _, tt := range tests {
		got := yamlEscape(tt.input)
		if got != tt.expected {
			t.Errorf("yamlEscape(%q): got %q, want %q", tt.input, got, tt.expected)
		}
	}
}
