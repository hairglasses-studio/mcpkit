package security

import (
	"encoding/json"
	"io"
	"sync"
)

// AuditExporter sends audit events to an external sink.
type AuditExporter interface {
	Export(event AuditEvent) error
	Close() error
}

// JSONLExporter writes audit events as newline-delimited JSON to an io.Writer.
// It is safe for concurrent use.
type JSONLExporter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewJSONLExporter creates a JSONLExporter that writes to w.
func NewJSONLExporter(w io.Writer) *JSONLExporter {
	return &JSONLExporter{w: w}
}

func (e *JSONLExporter) Export(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	e.mu.Lock()
	defer e.mu.Unlock()
	_, err = e.w.Write(data)
	return err
}

func (e *JSONLExporter) Close() error {
	if c, ok := e.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// FilterFunc returns true for events that should be exported.
type FilterFunc func(AuditEvent) bool

// StreamExporter wraps an io.Writer with an optional filter.
type StreamExporter struct {
	mu     sync.Mutex
	w      io.Writer
	filter FilterFunc
}

// NewStreamExporter creates a StreamExporter. If filter is nil, all events are exported.
func NewStreamExporter(w io.Writer, filter FilterFunc) *StreamExporter {
	return &StreamExporter{w: w, filter: filter}
}

func (e *StreamExporter) Export(event AuditEvent) error {
	if e.filter != nil && !e.filter(event) {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	e.mu.Lock()
	defer e.mu.Unlock()
	_, err = e.w.Write(data)
	return err
}

func (e *StreamExporter) Close() error {
	if c, ok := e.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
