package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ContextFormat specifies the output format for context serialization.
type ContextFormat string

const (
	// ContextFormatJSON serializes events as a JSON array.
	ContextFormatJSON ContextFormat = "json"
	// ContextFormatXML serializes events as XML with typed tags.
	ContextFormatXML ContextFormat = "xml"
	// ContextFormatYAML serializes events as compact YAML.
	ContextFormatYAML ContextFormat = "yaml"
	// ContextFormatCompact serializes events in a custom minimal format
	// optimized for token efficiency.
	ContextFormatCompact ContextFormat = "compact"
)

// EventFilter decides whether to include an event in context.
// Returning true means the event is included.
type EventFilter func(Event) bool

// ContextBuilder constructs optimized context representations from threads.
// It implements the "Own Your Context Window" principle (12-Factor Agent Factor 3)
// by producing format-specific, token-budgeted context from thread event logs.
type ContextBuilder struct {
	format    ContextFormat
	maxTokens int
	filters   []EventFilter
	lastN     int // 0 means no limit; >0 keeps only the last N events after filtering
}

// NewContextBuilder creates a builder with the given format and token budget.
// A maxTokens value of 0 or negative means no token limit.
func NewContextBuilder(format ContextFormat, maxTokens int) *ContextBuilder {
	return &ContextBuilder{
		format:    format,
		maxTokens: maxTokens,
	}
}

// WithFilter adds a per-event filter to the builder. Multiple filters are
// ANDed together: an event must pass all filters to be included.
// Returns the builder for chaining.
func (cb *ContextBuilder) WithFilter(f EventFilter) *ContextBuilder {
	cb.filters = append(cb.filters, f)
	return cb
}

// Build serializes the thread events into the specified format, respecting
// the token budget by truncating oldest events first. Thread-safe: reads
// thread state under lock via Replay().
func (cb *ContextBuilder) Build(thread *Thread) (string, error) {
	if thread == nil {
		return "", fmt.Errorf("session: cannot build context from nil thread")
	}

	events := thread.Replay()
	filtered := cb.applyFilters(events)

	// Apply lastN post-filter (keeps the tail).
	if cb.lastN > 0 && len(filtered) > cb.lastN {
		filtered = filtered[len(filtered)-cb.lastN:]
	}

	result, err := cb.render(filtered)
	if err != nil {
		return "", err
	}

	// If no token budget, return as-is.
	if cb.maxTokens <= 0 {
		return result, nil
	}

	// Truncate oldest events until within budget.
	for TokenCount(result) > cb.maxTokens && len(filtered) > 0 {
		filtered = filtered[1:]
		result, err = cb.render(filtered)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

// applyFilters returns events that pass all registered per-event filters.
func (cb *ContextBuilder) applyFilters(events []Event) []Event {
	if len(cb.filters) == 0 {
		return events
	}

	var out []Event
	for _, e := range events {
		if cb.passesAll(e) {
			out = append(out, e)
		}
	}
	return out
}

// passesAll checks whether an event passes all registered per-event filters.
func (cb *ContextBuilder) passesAll(e Event) bool {
	for _, f := range cb.filters {
		if !f(e) {
			return false
		}
	}
	return true
}

// render converts a slice of events to the configured format.
func (cb *ContextBuilder) render(events []Event) (string, error) {
	switch cb.format {
	case ContextFormatJSON:
		return renderJSON(events)
	case ContextFormatXML:
		return renderXML(events)
	case ContextFormatYAML:
		return renderYAML(events)
	case ContextFormatCompact:
		return renderCompact(events)
	default:
		return "", fmt.Errorf("session: unsupported context format: %q", cb.format)
	}
}

// contextEvent is the serialization representation of an event in context output.
type contextEvent struct {
	Type      EventType         `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      any               `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// toContextEvents converts events to their serialization representation.
func toContextEvents(events []Event) []contextEvent {
	out := make([]contextEvent, len(events))
	for i, e := range events {
		out[i] = contextEvent{
			Type:      e.Type,
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Data:      e.Data,
			Metadata:  e.Metadata,
		}
	}
	return out
}

// renderJSON renders events as a JSON array.
func renderJSON(events []Event) (string, error) {
	ce := toContextEvents(events)
	data, err := json.Marshal(ce)
	if err != nil {
		return "", fmt.Errorf("session: json marshal context: %w", err)
	}
	return string(data), nil
}

// renderXML renders events as XML with typed element tags.
func renderXML(events []Event) (string, error) {
	var b strings.Builder
	b.WriteString("<context>\n")
	for _, e := range events {
		tag := xmlSafeTag(string(e.Type))
		fmt.Fprintf(&b, "  <%s timestamp=%q>\n", tag, e.Timestamp.Format(time.RFC3339))
		if e.Data != nil {
			fmt.Fprintf(&b, "    <data>%s</data>\n", xmlEscape(fmt.Sprint(e.Data)))
		}
		if len(e.Metadata) > 0 {
			b.WriteString("    <metadata>\n")
			for k, v := range e.Metadata {
				fmt.Fprintf(&b, "      <%s>%s</%s>\n",
					xmlSafeTag(k), xmlEscape(v), xmlSafeTag(k))
			}
			b.WriteString("    </metadata>\n")
		}
		fmt.Fprintf(&b, "  </%s>\n", tag)
	}
	b.WriteString("</context>")
	return b.String(), nil
}

// renderYAML renders events as compact YAML.
func renderYAML(events []Event) (string, error) {
	var b strings.Builder
	for i, e := range events {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- type: %s\n", e.Type)
		fmt.Fprintf(&b, "  timestamp: %s\n", e.Timestamp.Format(time.RFC3339))
		if e.Data != nil {
			fmt.Fprintf(&b, "  data: %s\n", yamlEscape(fmt.Sprint(e.Data)))
		}
		if len(e.Metadata) > 0 {
			b.WriteString("  metadata:\n")
			for k, v := range e.Metadata {
				fmt.Fprintf(&b, "    %s: %s\n", k, yamlEscape(v))
			}
		}
	}
	return b.String(), nil
}

// renderCompact renders events in a custom minimal format optimized for
// token efficiency. Format: "[TYPE@TIME] DATA {k=v,...}"
func renderCompact(events []Event) (string, error) {
	var b strings.Builder
	for i, e := range events {
		if i > 0 {
			b.WriteString("\n")
		}
		ts := e.Timestamp.Format("15:04:05")
		fmt.Fprintf(&b, "[%s@%s]", e.Type, ts)
		if e.Data != nil {
			fmt.Fprintf(&b, " %s", fmt.Sprint(e.Data))
		}
		if len(e.Metadata) > 0 {
			b.WriteString(" {")
			first := true
			for k, v := range e.Metadata {
				if !first {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, "%s=%s", k, v)
				first = false
			}
			b.WriteString("}")
		}
	}
	return b.String(), nil
}

// xmlSafeTag converts a string to a valid XML tag name by replacing
// characters that are not valid in XML element names with underscores.
func xmlSafeTag(s string) string {
	var b strings.Builder
	for i, r := range s {
		if isXMLNameChar(r, i == 0) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	result := b.String()
	if result == "" {
		return "_"
	}
	return result
}

// isXMLNameChar reports whether r is valid in an XML name at the given position.
func isXMLNameChar(r rune, first bool) bool {
	if first {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
}

// xmlEscape replaces XML special characters with their entity references.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// yamlEscape quotes a string if it contains characters that need escaping in YAML.
func yamlEscape(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := false
	for _, r := range s {
		if r == ':' || r == '#' || r == '\n' || r == '"' || r == '\'' ||
			r == '{' || r == '}' || r == '[' || r == ']' || r == ',' ||
			r == '&' || r == '*' || r == '!' || r == '|' || r == '>' ||
			r == '%' || r == '@' || r == '`' {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		escaped := strings.ReplaceAll(s, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

// TokenCount estimates the token count of a string using a simple heuristic:
// approximately 4 characters per token for English text. This is a rough
// approximation suitable for budget enforcement; it is not a tokenizer.
func TokenCount(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4 // ceiling division
}

// --- Predefined Filters ---

// FilterByType returns a filter that includes only events matching one of
// the specified types.
func FilterByType(types ...EventType) EventFilter {
	set := make(map[EventType]bool, len(types))
	for _, t := range types {
		set[t] = true
	}
	return func(e Event) bool {
		return set[e.Type]
	}
}

// FilterExcludeType returns a filter that excludes events matching any of
// the specified types.
func FilterExcludeType(types ...EventType) EventFilter {
	set := make(map[EventType]bool, len(types))
	for _, t := range types {
		set[t] = true
	}
	return func(e Event) bool {
		return !set[e.Type]
	}
}

// FilterAfter returns a filter that includes only events with timestamps
// strictly after the given time.
func FilterAfter(t time.Time) EventFilter {
	return func(e Event) bool {
		return e.Timestamp.After(t)
	}
}

// FilterLastN configures the builder to keep only the last N events after
// all per-event filters have been applied. Unlike the other Filter* functions,
// this modifies the builder directly because it operates on the result set
// rather than individual events. Returns the builder for chaining.
func (cb *ContextBuilder) FilterLastN(n int) *ContextBuilder {
	cb.lastN = n
	return cb
}
