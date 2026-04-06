package session

import (
	"sync"
	"testing"
	"time"
)

func TestNewThread(t *testing.T) {
	th, err := NewThread()
	if err != nil {
		t.Fatalf("NewThread: %v", err)
	}
	if th.ID == "" {
		t.Fatal("expected non-empty thread ID")
	}
	if th.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	if len(th.Events) != 0 {
		t.Fatalf("expected empty events, got %d", len(th.Events))
	}
}

func TestNewThreadWithID(t *testing.T) {
	th := NewThreadWithID("custom-id")
	if th.ID != "custom-id" {
		t.Fatalf("got ID %q, want %q", th.ID, "custom-id")
	}
	if th.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestThreadAppend(t *testing.T) {
	th := NewThreadWithID("test-append")

	e1 := Event{
		ID:        "e1",
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Data:      "call-1",
	}
	e2 := Event{
		ID:        "e2",
		Type:      EventToolResult,
		Timestamp: time.Now(),
		Data:      "result-1",
	}

	th.Append(e1)
	th.Append(e2)

	if th.Len() != 2 {
		t.Fatalf("expected 2 events, got %d", th.Len())
	}
}

func TestThreadReplay(t *testing.T) {
	th := NewThreadWithID("test-replay")

	events := []Event{
		{ID: "e1", Type: EventToolCall, Timestamp: time.Now(), Data: "call"},
		{ID: "e2", Type: EventToolResult, Timestamp: time.Now(), Data: "result"},
		{ID: "e3", Type: EventCheckpoint, Timestamp: time.Now(), Data: "checkpoint"},
	}
	for _, e := range events {
		th.Append(e)
	}

	replayed := th.Replay()
	if len(replayed) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(replayed))
	}

	for i, e := range replayed {
		if e.ID != events[i].ID {
			t.Errorf("event %d: got ID %q, want %q", i, e.ID, events[i].ID)
		}
		if e.Type != events[i].Type {
			t.Errorf("event %d: got Type %q, want %q", i, e.Type, events[i].Type)
		}
	}

	// Verify returned slice is a copy — mutating it should not affect the thread.
	replayed[0].ID = "mutated"
	original := th.Replay()
	if original[0].ID == "mutated" {
		t.Fatal("Replay returned a reference, not a copy")
	}
}

func TestThreadReplayEmpty(t *testing.T) {
	th := NewThreadWithID("empty")
	replayed := th.Replay()
	if len(replayed) != 0 {
		t.Fatalf("expected 0 events, got %d", len(replayed))
	}
}

func TestThreadLast(t *testing.T) {
	th := NewThreadWithID("test-last")

	// Empty thread.
	_, ok := th.Last()
	if ok {
		t.Fatal("expected Last to return false for empty thread")
	}

	th.Append(Event{ID: "e1", Type: EventToolCall, Timestamp: time.Now()})
	th.Append(Event{ID: "e2", Type: EventToolResult, Timestamp: time.Now()})

	last, ok := th.Last()
	if !ok {
		t.Fatal("expected Last to return true")
	}
	if last.ID != "e2" {
		t.Fatalf("got last ID %q, want %q", last.ID, "e2")
	}
}

func TestThreadEventsByType(t *testing.T) {
	th := NewThreadWithID("test-by-type")
	th.Append(Event{ID: "e1", Type: EventToolCall, Timestamp: time.Now()})
	th.Append(Event{ID: "e2", Type: EventToolResult, Timestamp: time.Now()})
	th.Append(Event{ID: "e3", Type: EventToolCall, Timestamp: time.Now()})
	th.Append(Event{ID: "e4", Type: EventError, Timestamp: time.Now()})

	calls := th.EventsByType(EventToolCall)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].ID != "e1" || calls[1].ID != "e3" {
		t.Errorf("unexpected call IDs: %q, %q", calls[0].ID, calls[1].ID)
	}

	errors := th.EventsByType(EventError)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	humans := th.EventsByType(EventHumanRequest)
	if len(humans) != 0 {
		t.Fatalf("expected 0 human requests, got %d", len(humans))
	}
}

func TestThreadFork(t *testing.T) {
	th := NewThreadWithID("original")
	th.Append(Event{
		ID:        "e1",
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Data:      "call-data",
		Metadata:  map[string]string{"model": "opus"},
	})
	th.Append(Event{
		ID:        "e2",
		Type:      EventToolResult,
		Timestamp: time.Now(),
		Data:      "result-data",
	})

	forked := th.Fork("fork-1")

	// New ID.
	if forked.ID != "fork-1" {
		t.Fatalf("forked ID: got %q, want %q", forked.ID, "fork-1")
	}

	// Same events.
	if forked.Len() != th.Len() {
		t.Fatalf("forked events: got %d, want %d", forked.Len(), th.Len())
	}

	// Events are independent copies.
	forked.Append(Event{ID: "e3", Type: EventCheckpoint, Timestamp: time.Now()})
	if forked.Len() != 3 {
		t.Fatalf("forked should have 3 events, got %d", forked.Len())
	}
	if th.Len() != 2 {
		t.Fatal("original should still have 2 events after fork append")
	}

	// Metadata is deep-copied.
	forkedEvents := forked.Replay()
	forkedEvents[0].Metadata["model"] = "haiku"
	originalEvents := th.Replay()
	if originalEvents[0].Metadata["model"] != "opus" {
		t.Fatal("fork metadata mutation leaked to original")
	}
}

func TestThreadForkEmpty(t *testing.T) {
	th := NewThreadWithID("empty-original")
	forked := th.Fork("empty-fork")
	if forked.Len() != 0 {
		t.Fatalf("expected 0 events in forked empty thread, got %d", forked.Len())
	}
}

func TestThreadSerializeJSON(t *testing.T) {
	th := NewThreadWithID("json-thread")
	th.Append(Event{
		ID:        "e1",
		Type:      EventToolCall,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Data:      "test-data",
		Metadata:  map[string]string{"key": "value"},
	})

	data, err := th.Serialize(FormatJSON)
	if err != nil {
		t.Fatalf("Serialize JSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON data")
	}

	restored, err := Deserialize(data, FormatJSON)
	if err != nil {
		t.Fatalf("Deserialize JSON: %v", err)
	}

	if restored.ID != th.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, th.ID)
	}
	if len(restored.Events) != 1 {
		t.Fatalf("events: got %d, want 1", len(restored.Events))
	}
	if restored.Events[0].ID != "e1" {
		t.Errorf("event ID: got %q, want %q", restored.Events[0].ID, "e1")
	}
	if restored.Events[0].Type != EventToolCall {
		t.Errorf("event type: got %q, want %q", restored.Events[0].Type, EventToolCall)
	}
}

func TestThreadSerializeGob(t *testing.T) {
	th := NewThreadWithID("gob-thread")
	th.Append(Event{
		ID:        "e1",
		Type:      EventToolResult,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Data:      "gob-data",
		Metadata:  map[string]string{"format": "gob"},
	})

	data, err := th.Serialize(FormatGob)
	if err != nil {
		t.Fatalf("Serialize Gob: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty gob data")
	}

	restored, err := Deserialize(data, FormatGob)
	if err != nil {
		t.Fatalf("Deserialize Gob: %v", err)
	}

	if restored.ID != th.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, th.ID)
	}
	if len(restored.Events) != 1 {
		t.Fatalf("events: got %d, want 1", len(restored.Events))
	}
	if restored.Events[0].Type != EventToolResult {
		t.Errorf("event type: got %q, want %q", restored.Events[0].Type, EventToolResult)
	}
}

func TestThreadSerializeUnsupportedFormat(t *testing.T) {
	th := NewThreadWithID("bad-format")
	_, err := th.Serialize(Format(99))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestDeserializeEmptyData(t *testing.T) {
	_, err := Deserialize(nil, FormatJSON)
	if err == nil {
		t.Fatal("expected error for nil data")
	}
	_, err = Deserialize([]byte{}, FormatJSON)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestDeserializeUnsupportedFormat(t *testing.T) {
	_, err := Deserialize([]byte(`{}`), Format(99))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestDeserializeInvalidJSON(t *testing.T) {
	_, err := Deserialize([]byte(`not-json`), FormatJSON)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDeserializeInvalidGob(t *testing.T) {
	_, err := Deserialize([]byte(`not-gob`), FormatGob)
	if err == nil {
		t.Fatal("expected error for invalid gob data")
	}
}

func TestThreadSerializeRoundtripJSON(t *testing.T) {
	th := NewThreadWithID("roundtrip-json")
	now := time.Now().Truncate(time.Millisecond) // JSON loses nanoseconds
	for i := range 5 {
		th.Append(Event{
			ID:        idForIndex(i),
			Type:      EventToolCall,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Data:      "data",
			Metadata:  map[string]string{"i": idForIndex(i)},
		})
	}

	data, err := th.Serialize(FormatJSON)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	restored, err := Deserialize(data, FormatJSON)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if restored.Len() != 5 {
		t.Fatalf("expected 5 events, got %d", restored.Len())
	}

	for i, e := range restored.Replay() {
		if e.ID != idForIndex(i) {
			t.Errorf("event %d: ID got %q, want %q", i, e.ID, idForIndex(i))
		}
	}
}

func TestThreadSerializeRoundtripGob(t *testing.T) {
	th := NewThreadWithID("roundtrip-gob")
	now := time.Now()
	for i := range 5 {
		th.Append(Event{
			ID:        idForIndex(i),
			Type:      EventToolResult,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Data:      "data",
			Metadata:  map[string]string{"i": idForIndex(i)},
		})
	}

	data, err := th.Serialize(FormatGob)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	restored, err := Deserialize(data, FormatGob)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if restored.Len() != 5 {
		t.Fatalf("expected 5 events, got %d", restored.Len())
	}
}

func TestThreadConcurrency(t *testing.T) {
	th := NewThreadWithID("concurrent")

	const goroutines = 50
	const eventsPerGoroutine = 100
	var wg sync.WaitGroup

	// Concurrent appends.
	for g := range goroutines {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := range eventsPerGoroutine {
				th.Append(Event{
					ID:        idForGoroutine(gid, i),
					Type:      EventToolCall,
					Timestamp: time.Now(),
					Data:      "concurrent",
				})
			}
		}(g)
	}
	wg.Wait()

	total := goroutines * eventsPerGoroutine
	if th.Len() != total {
		t.Fatalf("expected %d events, got %d", total, th.Len())
	}

	// Concurrent reads during appends.
	wg.Add(goroutines * 2)
	for g := range goroutines {
		go func(gid int) {
			defer wg.Done()
			for range eventsPerGoroutine {
				th.Append(Event{
					ID:        idForGoroutine(gid+goroutines, 0),
					Type:      EventToolResult,
					Timestamp: time.Now(),
				})
			}
		}(g)
		go func() {
			defer wg.Done()
			for range eventsPerGoroutine {
				_ = th.Replay()
				_ = th.Len()
				_, _ = th.Last()
				_ = th.EventsByType(EventToolCall)
			}
		}()
	}
	wg.Wait()
}

func TestThreadConcurrentFork(t *testing.T) {
	th := NewThreadWithID("fork-parent")
	for i := range 100 {
		th.Append(Event{
			ID:        idForIndex(i),
			Type:      EventToolCall,
			Timestamp: time.Now(),
		})
	}

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(forkID int) {
			defer wg.Done()
			forked := th.Fork(idForIndex(forkID))
			if forked.Len() != 100 {
				t.Errorf("fork %d: expected 100 events, got %d", forkID, forked.Len())
			}
		}(i)
	}
	wg.Wait()
}

func TestThreadEmpty(t *testing.T) {
	th := NewThreadWithID("empty")

	if th.Len() != 0 {
		t.Fatalf("expected 0 length, got %d", th.Len())
	}

	events := th.Replay()
	if len(events) != 0 {
		t.Fatalf("expected empty replay, got %d events", len(events))
	}

	_, ok := th.Last()
	if ok {
		t.Fatal("expected Last to return false on empty thread")
	}

	calls := th.EventsByType(EventToolCall)
	if len(calls) != 0 {
		t.Fatalf("expected 0 events by type, got %d", len(calls))
	}

	// Serialize/deserialize empty thread.
	data, err := th.Serialize(FormatJSON)
	if err != nil {
		t.Fatalf("Serialize empty: %v", err)
	}
	restored, err := Deserialize(data, FormatJSON)
	if err != nil {
		t.Fatalf("Deserialize empty: %v", err)
	}
	if restored.Len() != 0 {
		t.Fatalf("restored empty thread has %d events", restored.Len())
	}
}

func TestThreadLargeEventCount(t *testing.T) {
	th := NewThreadWithID("large")
	const count = 10_000

	for i := range count {
		th.Append(Event{
			ID:        idForIndex(i),
			Type:      EventToolCall,
			Timestamp: time.Now(),
			Data:      i,
		})
	}

	if th.Len() != count {
		t.Fatalf("expected %d events, got %d", count, th.Len())
	}

	events := th.Replay()
	if len(events) != count {
		t.Fatalf("replay: expected %d events, got %d", count, len(events))
	}

	last, ok := th.Last()
	if !ok {
		t.Fatal("expected Last to return true")
	}
	if last.ID != idForIndex(count-1) {
		t.Fatalf("last event ID: got %q, want %q", last.ID, idForIndex(count-1))
	}
}

func TestNewEvent(t *testing.T) {
	e, err := NewEvent(EventToolCall, "test-payload")
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.ID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if e.Type != EventToolCall {
		t.Fatalf("expected type %q, got %q", EventToolCall, e.Type)
	}
	if e.Data != "test-payload" {
		t.Fatalf("expected data %q, got %v", "test-payload", e.Data)
	}
	if e.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if e.Metadata == nil {
		t.Fatal("expected non-nil metadata map")
	}
}

func TestEventTypes(t *testing.T) {
	// Ensure all event types are distinct non-empty strings.
	types := []EventType{
		EventToolCall,
		EventToolResult,
		EventError,
		EventHumanRequest,
		EventHumanResponse,
		EventCheckpoint,
		EventSystemMessage,
	}
	seen := make(map[EventType]bool)
	for _, typ := range types {
		if typ == "" {
			t.Fatal("event type should not be empty")
		}
		if seen[typ] {
			t.Fatalf("duplicate event type: %q", typ)
		}
		seen[typ] = true
	}
}

func TestAttachToSession(t *testing.T) {
	th := NewThreadWithID("attach-test")
	sess := newSession("sess-1", 0)

	th.AttachToSession(sess)

	id, ok := ThreadIDFromSession(sess)
	if !ok {
		t.Fatal("expected thread ID in session")
	}
	if id != "attach-test" {
		t.Fatalf("got thread ID %q, want %q", id, "attach-test")
	}
}

func TestThreadIDFromSession_Missing(t *testing.T) {
	sess := newSession("sess-2", 0)
	_, ok := ThreadIDFromSession(sess)
	if ok {
		t.Fatal("expected no thread ID in session")
	}
}

func TestThreadStore(t *testing.T) {
	store := NewThreadStore()

	if store.Len() != 0 {
		t.Fatalf("expected empty store, got %d", store.Len())
	}

	th1 := NewThreadWithID("t1")
	th2 := NewThreadWithID("t2")

	store.Put(th1)
	store.Put(th2)

	if store.Len() != 2 {
		t.Fatalf("expected 2 threads, got %d", store.Len())
	}

	got, ok := store.Get("t1")
	if !ok {
		t.Fatal("expected to find thread t1")
	}
	if got.ID != "t1" {
		t.Fatalf("got ID %q, want %q", got.ID, "t1")
	}

	_, ok = store.Get("nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent thread")
	}

	store.Delete("t1")
	if store.Len() != 1 {
		t.Fatalf("expected 1 thread after delete, got %d", store.Len())
	}
	_, ok = store.Get("t1")
	if ok {
		t.Fatal("expected t1 to be deleted")
	}
}

func TestThreadStoreConcurrency(t *testing.T) {
	store := NewThreadStore()
	var wg sync.WaitGroup

	const goroutines = 50
	for g := range goroutines {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			th := NewThreadWithID(idForIndex(gid))
			store.Put(th)
			store.Get(idForIndex(gid))
			store.Len()
		}(g)
	}
	wg.Wait()

	if store.Len() != goroutines {
		t.Fatalf("expected %d threads, got %d", goroutines, store.Len())
	}
}

// idForIndex returns a deterministic ID string for test indexing.
func idForIndex(i int) string {
	return "id-" + itoa(i)
}

// idForGoroutine returns a deterministic ID for goroutine+index.
func idForGoroutine(g, i int) string {
	return "g" + itoa(g) + "-" + itoa(i)
}

// itoa is a simple int-to-string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
