package session

import "time"

// EventType classifies the kind of event in a thread.
type EventType string

const (
	// EventToolCall represents an agent invoking a tool.
	EventToolCall EventType = "tool_call"
	// EventToolResult represents the result returned by a tool.
	EventToolResult EventType = "tool_result"
	// EventError represents an error that occurred during execution.
	EventError EventType = "error"
	// EventHumanRequest represents a request for human input (HITL).
	EventHumanRequest EventType = "human_request"
	// EventHumanResponse represents a human's response to a request.
	EventHumanResponse EventType = "human_response"
	// EventCheckpoint represents a snapshot/checkpoint in the workflow.
	EventCheckpoint EventType = "checkpoint"
	// EventSystemMessage represents a system-level message or annotation.
	EventSystemMessage EventType = "system_message"
)

// Event represents a single step in an agent thread. Events are the atomic
// units of the append-only log that forms a Thread's execution history
// (12-Factor Agent Factor 5).
type Event struct {
	// ID is the unique identifier for this event within its thread.
	ID string `json:"id"`
	// Type classifies the event (tool call, result, error, etc.).
	Type EventType `json:"type"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Data holds the event payload. The concrete type depends on EventType:
	// tool call parameters, tool results, error messages, human input, etc.
	Data any `json:"data"`
	// Metadata holds optional key-value annotations (e.g. model, latency, cost).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewEvent creates a new Event with the given type and data, timestamped to now.
// The ID is generated using a cryptographically random hex string.
// Returns an error only if ID generation fails.
func NewEvent(typ EventType, data any) (Event, error) {
	id, err := generateID()
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID:        id,
		Type:      typ,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]string),
	}, nil
}
