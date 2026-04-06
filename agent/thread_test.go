package agent

import (
	"testing"
	"time"
)

func TestNewThread(t *testing.T) {
	th := NewThread("test-001")
	if th.ID != "test-001" {
		t.Errorf("ID = %q, want %q", th.ID, "test-001")
	}
	if th.Len() != 0 {
		t.Errorf("Len() = %d, want 0", th.Len())
	}
	if th.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if th.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestReduce_AppendsEvent(t *testing.T) {
	th := NewThread("test-002")

	ev := Event{
		Type:      EventTypeToolCall,
		Timestamp: time.Now(),
		Data:      map[string]any{"tool": "read_file"},
		Metadata:  map[string]string{"task": "fetch"},
	}

	th2 := Reduce(th, ev)
	if th2.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", th2.Len())
	}
	if th2.Events[0].Sequence != 0 {
		t.Errorf("first event sequence = %d, want 0", th2.Events[0].Sequence)
	}
	if th2.Events[0].Event.Type != EventTypeToolCall {
		t.Errorf("event type = %q, want %q", th2.Events[0].Event.Type, EventTypeToolCall)
	}
}

func TestReduce_PreservesImmutability(t *testing.T) {
	th := NewThread("test-003")
	ev1 := Event{Type: EventTypeToolCall, Timestamp: time.Now()}
	th2 := Reduce(th, ev1)

	// Original thread should be unchanged.
	if th.Len() != 0 {
		t.Errorf("original thread modified: Len() = %d, want 0", th.Len())
	}
	if th2.Len() != 1 {
		t.Errorf("new thread Len() = %d, want 1", th2.Len())
	}
}

func TestReduce_SequenceIncrement(t *testing.T) {
	th := NewThread("test-004")
	for i := 0; i < 5; i++ {
		th = Reduce(th, Event{Type: EventTypeToolCall, Timestamp: time.Now()})
	}
	if th.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", th.Len())
	}
	for i, te := range th.Events {
		if te.Sequence != i {
			t.Errorf("event[%d].Sequence = %d, want %d", i, te.Sequence, i)
		}
	}
}

func TestThread_Last_Empty(t *testing.T) {
	th := NewThread("test-005")
	_, ok := th.Last()
	if ok {
		t.Error("Last() on empty thread should return false")
	}
}

func TestThread_Last_NonEmpty(t *testing.T) {
	th := NewThread("test-006")
	th = Reduce(th, Event{Type: EventTypeToolCall, Timestamp: time.Now()})
	th = Reduce(th, Event{Type: EventTypeLLMResponse, Timestamp: time.Now()})

	last, ok := th.Last()
	if !ok {
		t.Fatal("Last() should return true for non-empty thread")
	}
	if last.Type != EventTypeLLMResponse {
		t.Errorf("Last().Type = %q, want %q", last.Type, EventTypeLLMResponse)
	}
}

func TestThread_EventsByType(t *testing.T) {
	th := NewThread("test-007")
	th = Reduce(th, Event{Type: EventTypeToolCall, Timestamp: time.Now()})
	th = Reduce(th, Event{Type: EventTypeLLMResponse, Timestamp: time.Now()})
	th = Reduce(th, Event{Type: EventTypeToolCall, Timestamp: time.Now()})
	th = Reduce(th, Event{Type: EventTypeError, Timestamp: time.Now()})

	toolCalls := th.EventsByType(EventTypeToolCall)
	if len(toolCalls) != 2 {
		t.Errorf("EventsByType(tool_call) = %d events, want 2", len(toolCalls))
	}

	errors := th.EventsByType(EventTypeError)
	if len(errors) != 1 {
		t.Errorf("EventsByType(error) = %d events, want 1", len(errors))
	}

	missing := th.EventsByType("nonexistent")
	if len(missing) != 0 {
		t.Errorf("EventsByType(nonexistent) = %d events, want 0", len(missing))
	}
}

func TestThread_ID_Preserved(t *testing.T) {
	th := NewThread("preserve-id")
	th = Reduce(th, Event{Type: EventTypeToolCall, Timestamp: time.Now()})
	if th.ID != "preserve-id" {
		t.Errorf("ID = %q, want %q after Reduce", th.ID, "preserve-id")
	}
}

func TestEvent_Constants(t *testing.T) {
	// Verify constants are defined and distinct.
	types := []string{
		EventTypeToolCall,
		EventTypeToolResult,
		EventTypeLLMRequest,
		EventTypeLLMResponse,
		EventTypeError,
		EventTypeHumanInput,
		EventTypeHumanOutput,
		EventTypeStateChange,
		EventTypeCheckpoint,
		EventTypePreFetch,
	}

	seen := make(map[string]bool)
	for _, typ := range types {
		if typ == "" {
			t.Error("event type constant should not be empty")
		}
		if seen[typ] {
			t.Errorf("duplicate event type constant: %q", typ)
		}
		seen[typ] = true
	}
}
