//go:build !official_sdk

package hitools

import (
	"fmt"
	"sync"
)

// PendingRequest is a request awaiting human response. It pairs the original
// RequestInput with tracking metadata so callers can list, inspect, and
// complete pending interactions.
type PendingRequest struct {
	// ID uniquely identifies this pending request.
	ID string `json:"id"`
	// Input is the original human-input request parameters.
	Input RequestInput `json:"input"`
	// CreatedAt is an ISO 8601 timestamp of when the request was created.
	CreatedAt string `json:"created_at"`
}

// ResponseStore manages the lifecycle of human interaction requests.
// Implementations must be safe for concurrent use.
type ResponseStore interface {
	// Save stores a pending request. If a request with the same ID already
	// exists, it is overwritten.
	Save(req PendingRequest) error
	// Load retrieves a pending request by ID. Returns (zero, false, nil) if
	// the request does not exist.
	Load(id string) (PendingRequest, bool, error)
	// Complete marks a pending request as completed with the given output and
	// moves it from the pending set to the completed set.
	Complete(id string, output RequestOutput) error
	// ListPending returns the IDs of all pending requests.
	ListPending() ([]string, error)
	// Delete removes a pending request without completing it.
	Delete(id string) error
	// GetCompleted retrieves the output for a completed request. Returns
	// (zero, false) if the request was never completed.
	GetCompleted(id string) (RequestOutput, bool)
}

// InMemoryResponseStore is a thread-safe in-memory implementation of
// ResponseStore. Suitable for single-process use cases and testing.
type InMemoryResponseStore struct {
	mu        sync.RWMutex
	pending   map[string]PendingRequest
	completed map[string]RequestOutput
}

// NewInMemoryResponseStore creates a new empty in-memory response store.
func NewInMemoryResponseStore() *InMemoryResponseStore {
	return &InMemoryResponseStore{
		pending:   make(map[string]PendingRequest),
		completed: make(map[string]RequestOutput),
	}
}

// Save stores a pending request, overwriting any existing request with the same ID.
func (s *InMemoryResponseStore) Save(req PendingRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[req.ID] = req
	return nil
}

// Load retrieves a pending request by ID.
func (s *InMemoryResponseStore) Load(id string) (PendingRequest, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.pending[id]
	return req, ok, nil
}

// Complete marks a request as completed and moves it from pending to completed.
func (s *InMemoryResponseStore) Complete(id string, output RequestOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[id]; !ok {
		return fmt.Errorf("hitools: pending request %q not found", id)
	}
	delete(s.pending, id)
	s.completed[id] = output
	return nil
}

// ListPending returns the IDs of all pending requests.
func (s *InMemoryResponseStore) ListPending() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.pending))
	for id := range s.pending {
		ids = append(ids, id)
	}
	return ids, nil
}

// Delete removes a pending request without completing it.
func (s *InMemoryResponseStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
	return nil
}

// GetCompleted retrieves the output for a completed request.
func (s *InMemoryResponseStore) GetCompleted(id string) (RequestOutput, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out, ok := s.completed[id]
	return out, ok
}
