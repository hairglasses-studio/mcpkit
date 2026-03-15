package workflow

import (
	"context"
	"sync"
	"time"
)

// Checkpoint represents the saved state of a workflow at a point in time.
type Checkpoint struct {
	RunID       string    `json:"run_id"`
	State       State     `json:"state"`
	CurrentNode string    `json:"current_node"`
	Step        int       `json:"step"`
	SavedAt     time.Time `json:"saved_at"`
}

// CheckpointStore provides pluggable persistence for workflow checkpoints.
type CheckpointStore interface {
	Save(ctx context.Context, cp Checkpoint) error
	Load(ctx context.Context, runID string) (Checkpoint, bool, error)
	Delete(ctx context.Context, runID string) error
	List(ctx context.Context) ([]string, error)
}

// MemoryCheckpointStore is an in-memory implementation of CheckpointStore.
type MemoryCheckpointStore struct {
	mu          sync.RWMutex
	checkpoints map[string]Checkpoint
}

// NewMemoryCheckpointStore creates a new in-memory checkpoint store.
func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{
		checkpoints: make(map[string]Checkpoint),
	}
}

func (s *MemoryCheckpointStore) Save(_ context.Context, cp Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[cp.RunID] = cp
	return nil
}

func (s *MemoryCheckpointStore) Load(_ context.Context, runID string) (Checkpoint, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.checkpoints[runID]
	return cp, ok, nil
}

func (s *MemoryCheckpointStore) Delete(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkpoints, runID)
	return nil
}

func (s *MemoryCheckpointStore) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.checkpoints))
	for id := range s.checkpoints {
		ids = append(ids, id)
	}
	return ids, nil
}
