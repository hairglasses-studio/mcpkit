package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Sink stores submitted feedback records.
type Sink interface {
	WriteFeedback(ctx context.Context, record Record) error
}

// SinkFunc adapts a function into a Sink.
type SinkFunc func(ctx context.Context, record Record) error

// WriteFeedback stores a feedback record.
func (f SinkFunc) WriteFeedback(ctx context.Context, record Record) error {
	return f(ctx, record)
}

// MemorySink stores feedback in memory. It is safe for concurrent use.
type MemorySink struct {
	mu      sync.Mutex
	records []Record
}

// NewMemorySink creates an empty in-memory feedback sink.
func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

// WriteFeedback stores a feedback record in memory.
func (s *MemorySink) WriteFeedback(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

// Records returns a copy of all stored records.
func (s *MemorySink) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out
}

// JSONLSink appends feedback records to a JSON Lines file. It is safe for
// concurrent use.
type JSONLSink struct {
	path string
	mu   sync.Mutex
}

// NewJSONLSink creates a JSON Lines file sink at path. Parent directories are
// created on first write.
func NewJSONLSink(path string) (*JSONLSink, error) {
	if path == "" {
		return nil, fmt.Errorf("feedback jsonl path must not be empty")
	}
	return &JSONLSink{path: path}, nil
}

// Path returns the configured JSON Lines file path.
func (s *JSONLSink) Path() string {
	return s.path
}

// WriteFeedback appends one JSON object and newline to the sink file.
func (s *JSONLSink) WriteFeedback(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal feedback record: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create feedback sink directory: %w", err)
		}
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open feedback sink: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write feedback record: %w", err)
	}
	return nil
}
