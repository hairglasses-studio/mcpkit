package session

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Format specifies the serialization format for threads.
type Format int

const (
	// FormatJSON serializes threads as JSON.
	FormatJSON Format = iota
	// FormatGob serializes threads using Go's gob encoding.
	FormatGob
)

// Thread represents an append-only sequence of events for an agent session.
// It is the single source of truth for execution state (12-Factor Agent Factor 5).
//
// Threads enable pause/resume, replay, forking, and debugging by maintaining a
// complete, ordered history of every action taken during an agent session.
type Thread struct {
	// ID is the unique identifier for this thread.
	ID string `json:"id"`
	// CreatedAt is when the thread was created.
	CreatedAt time.Time `json:"created_at"`
	// Events is the append-only log of events.
	Events []Event `json:"events"`

	mu sync.RWMutex
}

// threadKey is the session data key used to store a thread ID in a session.
const threadKey = "_thread_id"

// NewThread creates a new empty thread with a random ID.
func NewThread() (*Thread, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("session: generate thread ID: %w", err)
	}
	return &Thread{
		ID:        id,
		CreatedAt: time.Now(),
		Events:    make([]Event, 0),
	}, nil
}

// NewThreadWithID creates a new empty thread with the specified ID.
func NewThreadWithID(id string) *Thread {
	return &Thread{
		ID:        id,
		CreatedAt: time.Now(),
		Events:    make([]Event, 0),
	}
}

// Append adds an event to the thread. This is the only way to add events,
// enforcing the append-only invariant. Thread-safe.
func (t *Thread) Append(e Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Events = append(t.Events, e)
}

// Replay returns a copy of all events in chronological order. Thread-safe.
// The returned slice is a snapshot; mutations to it do not affect the thread.
func (t *Thread) Replay() []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Event, len(t.Events))
	copy(out, t.Events)
	return out
}

// Len returns the number of events in the thread. Thread-safe.
func (t *Thread) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Events)
}

// Last returns the most recent event, or false if the thread is empty.
// Thread-safe.
func (t *Thread) Last() (Event, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Events) == 0 {
		return Event{}, false
	}
	return t.Events[len(t.Events)-1], true
}

// EventsByType returns all events matching the given type. Thread-safe.
func (t *Thread) EventsByType(typ EventType) []Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Event
	for _, e := range t.Events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// Fork creates a deep copy of the thread with a new ID, preserving the full
// event history. This enables branching workflows where an agent can explore
// alternative execution paths. Thread-safe.
func (t *Thread) Fork(newID string) *Thread {
	t.mu.RLock()
	defer t.mu.RUnlock()
	events := make([]Event, len(t.Events))
	for i, e := range t.Events {
		events[i] = Event{
			ID:        e.ID,
			Type:      e.Type,
			Timestamp: e.Timestamp,
			Data:      e.Data,
		}
		if e.Metadata != nil {
			events[i].Metadata = make(map[string]string, len(e.Metadata))
			for k, v := range e.Metadata {
				events[i].Metadata[k] = v
			}
		}
	}
	return &Thread{
		ID:        newID,
		CreatedAt: time.Now(),
		Events:    events,
	}
}

// serializableThread is the wire format for thread serialization.
// It mirrors Thread but without the mutex, making it safe for encoding.
type serializableThread struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Events    []Event   `json:"events"`
}

// Serialize converts the thread to a portable byte representation in the
// specified format. Thread-safe.
func (t *Thread) Serialize(format Format) ([]byte, error) {
	t.mu.RLock()
	st := serializableThread{
		ID:        t.ID,
		CreatedAt: t.CreatedAt,
		Events:    make([]Event, len(t.Events)),
	}
	copy(st.Events, t.Events)
	t.mu.RUnlock()

	switch format {
	case FormatJSON:
		return json.Marshal(st)
	case FormatGob:
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(st); err != nil {
			return nil, fmt.Errorf("session: gob encode thread: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("session: unsupported format: %d", format)
	}
}

// Deserialize restores a thread from serialized data in the specified format.
func Deserialize(data []byte, format Format) (*Thread, error) {
	if len(data) == 0 {
		return nil, errors.New("session: cannot deserialize empty data")
	}

	var st serializableThread
	switch format {
	case FormatJSON:
		if err := json.Unmarshal(data, &st); err != nil {
			return nil, fmt.Errorf("session: json unmarshal thread: %w", err)
		}
	case FormatGob:
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&st); err != nil {
			return nil, fmt.Errorf("session: gob decode thread: %w", err)
		}
	default:
		return nil, fmt.Errorf("session: unsupported format: %d", format)
	}

	return &Thread{
		ID:        st.ID,
		CreatedAt: st.CreatedAt,
		Events:    st.Events,
	}, nil
}

// AttachToSession stores the thread ID in the session's key-value store,
// creating a link between the session and its thread.
func (t *Thread) AttachToSession(sess Session) {
	sess.Set(threadKey, t.ID)
}

// ThreadIDFromSession retrieves the thread ID from a session, if one is attached.
func ThreadIDFromSession(sess Session) (string, bool) {
	v, ok := sess.Get(threadKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

// ThreadStore manages thread persistence alongside the session store.
type ThreadStore struct {
	mu      sync.RWMutex
	threads map[string]*Thread
}

// NewThreadStore creates a new in-memory thread store.
func NewThreadStore() *ThreadStore {
	return &ThreadStore{
		threads: make(map[string]*Thread),
	}
}

// Put stores a thread. If a thread with the same ID exists, it is replaced.
func (ts *ThreadStore) Put(t *Thread) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.threads[t.ID] = t
}

// Get retrieves a thread by ID. Returns (nil, false) if not found.
func (ts *ThreadStore) Get(id string) (*Thread, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	t, ok := ts.threads[id]
	return t, ok
}

// Delete removes a thread by ID.
func (ts *ThreadStore) Delete(id string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.threads, id)
}

// Len returns the number of threads in the store.
func (ts *ThreadStore) Len() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.threads)
}

// All returns a snapshot of all threads in the store. The returned slice is
// safe to iterate without holding the store lock. Thread-safe.
func (ts *ThreadStore) All() []*Thread {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]*Thread, 0, len(ts.threads))
	for _, t := range ts.threads {
		out = append(out, t)
	}
	return out
}
