//go:build !official_sdk

// Package hitools provides human interaction MCP tools built on mcpkit's
// elicitation primitives. The primary tool, request_human_input, lets an
// agent ask the connected human a question via the MCP elicitation protocol.
package hitools

// Urgency controls notification channel routing.
type Urgency string

const (
	UrgencyLow    Urgency = "low"
	UrgencyMedium Urgency = "medium"
	UrgencyHigh   Urgency = "high"
)

// ResponseFormat controls the elicitation UI presented to the human.
type ResponseFormat string

const (
	FormatFreeText       ResponseFormat = "free_text"
	FormatYesNo          ResponseFormat = "yes_no"
	FormatMultipleChoice ResponseFormat = "multiple_choice"
)

// RequestInput is the input schema for the request_human_input tool.
type RequestInput struct {
	Question string         `json:"question" jsonschema:"required,description=Question to ask the human"`
	Context  string         `json:"context,omitempty" jsonschema:"description=Decision context for the human"`
	Urgency  Urgency        `json:"urgency,omitempty" jsonschema:"enum=low,enum=medium,enum=high,description=Notification urgency"`
	Format   ResponseFormat `json:"format,omitempty" jsonschema:"enum=free_text,enum=yes_no,enum=multiple_choice,description=Expected response format"`
	Choices  []string       `json:"choices,omitempty" jsonschema:"description=Options for multiple_choice format"`
	Timeout  int            `json:"timeout_seconds,omitempty" jsonschema:"description=Max wait time in seconds. 0 means infinite"`
}

// RequestOutput is the structured response from a human interaction request.
type RequestOutput struct {
	Status    string `json:"status"`    // "accepted", "declined", "timeout"
	Response  string `json:"response"`  // The human's answer
	Timestamp string `json:"timestamp"` // ISO 8601
}
