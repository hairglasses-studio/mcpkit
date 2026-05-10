package feedback

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

const (
	// ToolName is the MCP tool name exposed by Module.
	ToolName = "feedback_submit"

	defaultMaxMessageChars = 4000
)

// Severity classifies feedback urgency.
type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// SubmitInput is the typed input schema for feedback_submit.
type SubmitInput struct {
	Category     string            `json:"category" jsonschema:"required,description=Feedback category such as bug feature docs or usability"`
	Message      string            `json:"message" jsonschema:"required,description=Plain-language feedback message"`
	Severity     Severity          `json:"severity,omitempty" jsonschema:"description=Feedback severity,enum=info,enum=low,enum=medium,enum=high"`
	Rating       int               `json:"rating,omitempty" jsonschema:"description=Optional 1 to 5 satisfaction rating,minimum=1,maximum=5"`
	Tags         []string          `json:"tags,omitempty" jsonschema:"description=Optional short tags for grouping feedback"`
	Context      map[string]string `json:"context,omitempty" jsonschema:"description=Optional non-sensitive context such as tool name or workflow id"`
	AllowContact bool              `json:"allow_contact,omitempty" jsonschema:"description=Whether the user permits follow-up contact"`
	Contact      string            `json:"contact,omitempty" jsonschema:"description=Optional contact handle only when allow_contact is true"`
}

// SubmitOutput is returned by feedback_submit after the sink accepts a record.
type SubmitOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Stored bool   `json:"stored"`
}

// Record is the normalized feedback stored in sinks.
type Record struct {
	ID           string            `json:"id"`
	Category     string            `json:"category"`
	Message      string            `json:"message"`
	Severity     Severity          `json:"severity"`
	Rating       int               `json:"rating,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Context      map[string]string `json:"context,omitempty"`
	AllowContact bool              `json:"allow_contact,omitempty"`
	Contact      string            `json:"contact,omitempty"`
	Source       string            `json:"source,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Module exposes feedback_submit as a registry.ToolModule.
type Module struct {
	sink            Sink
	source          string
	now             func() time.Time
	newID           func() string
	maxMessageChars int
}

// Option configures a Module.
type Option func(*Module)

// WithSource records the server or product name that produced feedback.
func WithSource(source string) Option {
	return func(m *Module) {
		m.source = strings.TrimSpace(source)
	}
}

// WithClock overrides the timestamp source. It is primarily useful in tests.
func WithClock(now func() time.Time) Option {
	return func(m *Module) {
		if now != nil {
			m.now = now
		}
	}
}

// WithIDGenerator overrides feedback ID generation. It is primarily useful in tests.
func WithIDGenerator(newID func() string) Option {
	return func(m *Module) {
		if newID != nil {
			m.newID = newID
		}
	}
}

// WithMaxMessageChars overrides the maximum feedback message length.
func WithMaxMessageChars(max int) Option {
	return func(m *Module) {
		if max > 0 {
			m.maxMessageChars = max
		}
	}
}

// NewModule creates a feedback module. If sink is nil, an in-memory sink is
// used so local demos remain functional.
func NewModule(sink Sink, opts ...Option) *Module {
	if sink == nil {
		sink = NewMemorySink()
	}
	m := &Module{
		sink:            sink,
		now:             time.Now,
		newID:           uuid.NewString,
		maxMessageChars: defaultMaxMessageChars,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Name returns the module name.
func (m *Module) Name() string { return "feedback" }

// Description returns the module description.
func (m *Module) Description() string { return "Structured user feedback collection" }

// Tools returns feedback_submit.
func (m *Module) Tools() []registry.ToolDefinition {
	td := handler.TypedHandler[SubmitInput, SubmitOutput](
		ToolName,
		"Collect structured product or workflow feedback and write it to the configured feedback sink.",
		m.submit,
	)
	td.Category = "feedback"
	td.Tags = []string{"feedback", "telemetry", "issue-reporting"}
	td.Complexity = registry.ComplexitySimple
	td.IsWrite = true
	return []registry.ToolDefinition{td}
}

func (m *Module) submit(ctx context.Context, input SubmitInput) (SubmitOutput, error) {
	if err := m.validate(input); err != nil {
		return SubmitOutput{}, err
	}

	record := Record{
		ID:           m.newID(),
		Category:     strings.TrimSpace(input.Category),
		Message:      strings.TrimSpace(input.Message),
		Severity:     normalizeSeverity(input.Severity),
		Rating:       input.Rating,
		Tags:         compactStrings(input.Tags),
		Context:      compactMap(input.Context),
		AllowContact: input.AllowContact,
		Contact:      strings.TrimSpace(input.Contact),
		Source:       m.source,
		CreatedAt:    m.now().UTC(),
	}
	if err := m.sink.WriteFeedback(ctx, record); err != nil {
		return SubmitOutput{}, fmt.Errorf("write feedback: %w", err)
	}
	return SubmitOutput{ID: record.ID, Status: "stored", Stored: true}, nil
}

func (m *Module) validate(input SubmitInput) error {
	if strings.TrimSpace(input.Category) == "" {
		return fmt.Errorf("category is required")
	}
	if strings.TrimSpace(input.Message) == "" {
		return fmt.Errorf("message is required")
	}
	if utf8.RuneCountInString(input.Message) > m.maxMessageChars {
		return fmt.Errorf("message exceeds %d characters", m.maxMessageChars)
	}
	if input.Rating != 0 && (input.Rating < 1 || input.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	if _, ok := validSeverity(input.Severity); !ok {
		return fmt.Errorf("severity must be one of info, low, medium, or high")
	}
	if strings.TrimSpace(input.Contact) != "" && !input.AllowContact {
		return fmt.Errorf("contact requires allow_contact=true")
	}
	return nil
}

func normalizeSeverity(severity Severity) Severity {
	if severity == "" {
		return SeverityInfo
	}
	return severity
}

func validSeverity(severity Severity) (Severity, bool) {
	switch normalizeSeverity(severity) {
	case SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh:
		return normalizeSeverity(severity), true
	default:
		return "", false
	}
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func compactMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
