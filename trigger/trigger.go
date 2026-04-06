// Package trigger provides types and a registry for managing event sources
// that initiate agent actions. A TriggerSource represents an external event
// producer (webhook, cron schedule, file watcher, etc.) that can start or
// resume agent work.
//
// The TriggerRecord captures when and why an agent action was triggered,
// providing full auditability for 12-Factor Agent compliance.
package trigger

import (
	"sync"
	"time"
)

// TriggerSource is the interface for event sources that can initiate agent work.
type TriggerSource interface {
	// Name returns a unique identifier for this trigger source.
	Name() string
	// Type returns the category of trigger (e.g., "webhook", "cron", "manual", "file_watch").
	Type() string
	// Description returns a human-readable description of the trigger.
	Description() string
	// Active reports whether the trigger source is currently enabled.
	Active() bool
}

// TriggerRecord captures a single trigger event for audit and replay.
type TriggerRecord struct {
	// ID is a unique identifier for this trigger event.
	ID string `json:"id"`
	// Source is the name of the TriggerSource that fired.
	Source string `json:"source"`
	// Type is the trigger source type (e.g., "webhook", "cron").
	Type string `json:"type"`
	// Timestamp is when the trigger fired.
	Timestamp time.Time `json:"timestamp"`
	// Payload is optional data associated with the trigger event.
	Payload map[string]any `json:"payload,omitempty"`
	// Metadata contains optional key-value pairs for filtering and routing.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Registry manages trigger source registration and lookup.
type Registry struct {
	mu      sync.RWMutex
	sources map[string]TriggerSource
	records []TriggerRecord
}

// NewRegistry creates a new trigger registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[string]TriggerSource),
	}
}

// Register adds a trigger source to the registry.
func (r *Registry) Register(source TriggerSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[source.Name()] = source
}

// Unregister removes a trigger source from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, name)
}

// Get returns a trigger source by name.
func (r *Registry) Get(name string) (TriggerSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[name]
	return s, ok
}

// List returns all registered trigger source names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.sources))
	for name := range r.sources {
		names = append(names, name)
	}
	return names
}

// ListActive returns names of trigger sources that are currently active.
func (r *Registry) ListActive() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, src := range r.sources {
		if src.Active() {
			names = append(names, name)
		}
	}
	return names
}

// RecordTrigger appends a trigger record to the audit log.
func (r *Registry) RecordTrigger(record TriggerRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
}

// Records returns all recorded trigger events.
func (r *Registry) Records() []TriggerRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TriggerRecord, len(r.records))
	copy(out, r.records)
	return out
}

// RecordsSince returns trigger records after the given timestamp.
func (r *Registry) RecordsSince(since time.Time) []TriggerRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []TriggerRecord
	for _, rec := range r.records {
		if rec.Timestamp.After(since) {
			out = append(out, rec)
		}
	}
	return out
}

// Count returns the number of registered trigger sources.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sources)
}

// StaticSource is a simple TriggerSource implementation for testing and
// configuration-driven triggers.
type StaticSource struct {
	SourceName        string
	SourceType        string
	SourceDescription string
	IsActive          bool
}

// Name returns the source name.
func (s *StaticSource) Name() string { return s.SourceName }

// Type returns the source type.
func (s *StaticSource) Type() string { return s.SourceType }

// Description returns the source description.
func (s *StaticSource) Description() string { return s.SourceDescription }

// Active reports whether the source is active.
func (s *StaticSource) Active() bool { return s.IsActive }
