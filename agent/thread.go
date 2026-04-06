// Package agent provides core types for agent execution threads and events.
//
// A Thread is an append-only event log that records the complete history of
// an agent's execution. Events represent discrete state changes (tool calls,
// LLM responses, errors, human input, etc.). The Reduce function applies an
// event to a thread, producing a new thread state.
//
// This package follows the 12-Factor Agents pattern of maintaining an
// immutable, append-only execution log that can be replayed for debugging,
// auditing, and recovery.
package agent

import (
	"time"
)

// Standard event types for agent threads.
const (
	EventTypeToolCall     = "tool_call"
	EventTypeToolResult   = "tool_result"
	EventTypeLLMRequest   = "llm_request"
	EventTypeLLMResponse  = "llm_response"
	EventTypeError        = "error"
	EventTypeHumanInput   = "human_input"
	EventTypeHumanOutput  = "human_output"
	EventTypeStateChange  = "state_change"
	EventTypeCheckpoint   = "checkpoint"
	EventTypePreFetch     = "pre_fetch"
)

// Event represents a discrete occurrence in an agent's execution.
// Events are immutable once created and form the building blocks of a Thread.
type Event struct {
	// Type classifies the event (e.g., "tool_call", "llm_response", "error").
	Type string `json:"type"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Data is the event payload. The concrete type depends on Event.Type.
	Data any `json:"data,omitempty"`
	// Metadata contains optional key-value pairs for tagging, filtering, and routing.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ThreadEvent is a positioned event within a thread, combining the event
// with its sequence number for ordering guarantees.
type ThreadEvent struct {
	// Sequence is the monotonically increasing position in the thread.
	Sequence int `json:"sequence"`
	// Event is the actual event data.
	Event Event `json:"event"`
}

// Thread is an append-only event log representing an agent's execution history.
// It is the canonical state representation for 12-Factor Agent threads.
type Thread struct {
	// ID uniquely identifies this thread.
	ID string `json:"id"`
	// Events is the ordered list of thread events.
	Events []ThreadEvent `json:"events"`
	// CreatedAt is when the thread was created.
	CreatedAt time.Time `json:"created_at"`
	// Metadata contains optional thread-level key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewThread creates a new empty thread with the given ID.
func NewThread(id string) Thread {
	return Thread{
		ID:        id,
		Events:    nil,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// Reduce appends an event to a thread, returning the updated thread.
// The thread is treated as an append-only log: existing events are never
// modified. The new event receives the next sequence number automatically.
func Reduce(thread Thread, event Event) Thread {
	seq := 0
	if len(thread.Events) > 0 {
		seq = thread.Events[len(thread.Events)-1].Sequence + 1
	}

	// Copy events to preserve immutability of the input.
	newEvents := make([]ThreadEvent, len(thread.Events), len(thread.Events)+1)
	copy(newEvents, thread.Events)
	newEvents = append(newEvents, ThreadEvent{
		Sequence: seq,
		Event:    event,
	})

	return Thread{
		ID:        thread.ID,
		Events:    newEvents,
		CreatedAt: thread.CreatedAt,
		Metadata:  thread.Metadata,
	}
}

// Len returns the number of events in the thread.
func (t Thread) Len() int {
	return len(t.Events)
}

// Last returns the most recent event, or a zero Event if the thread is empty.
func (t Thread) Last() (Event, bool) {
	if len(t.Events) == 0 {
		return Event{}, false
	}
	return t.Events[len(t.Events)-1].Event, true
}

// EventsByType returns all events of the given type.
func (t Thread) EventsByType(eventType string) []ThreadEvent {
	var result []ThreadEvent
	for _, te := range t.Events {
		if te.Event.Type == eventType {
			result = append(result, te)
		}
	}
	return result
}
