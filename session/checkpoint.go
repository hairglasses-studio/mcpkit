package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ThreadStatus represents the current lifecycle state of a thread.
type ThreadStatus string

const (
	// StatusRunning indicates the thread is actively executing.
	StatusRunning ThreadStatus = "running"
	// StatusPaused indicates the thread has been paused at a tool-call boundary.
	StatusPaused ThreadStatus = "paused"
	// StatusCompleted indicates the thread finished successfully.
	StatusCompleted ThreadStatus = "completed"
	// StatusFailed indicates the thread terminated with an error.
	StatusFailed ThreadStatus = "failed"
)

// checkpointReasonKey is the metadata key used to store the pause reason.
const checkpointReasonKey = "reason"

// checkpointStatusKey is the metadata key used to store the thread status at checkpoint time.
const checkpointStatusKey = "status"

// CheckpointManager handles saving and restoring agent thread state.
// It implements the pause/resume checkpoint API described by 12-Factor Agent
// Factor 6: "Launch/Pause/Resume with Simple APIs". Agents can cleanly break
// at tool-call boundaries, persist state, and resume without losing context.
type CheckpointManager struct {
	store  *ThreadStore
	format Format
	mu     sync.RWMutex
}

// NewCheckpointManager creates a manager backed by the given ThreadStore.
func NewCheckpointManager(store *ThreadStore, format Format) *CheckpointManager {
	return &CheckpointManager{
		store:  store,
		format: format,
	}
}

// Save persists the current thread state to the backing store.
// Returns the thread ID which serves as the checkpoint identifier.
func (cm *CheckpointManager) Save(_ context.Context, thread *Thread) (string, error) {
	if thread == nil {
		return "", errors.New("session: cannot save nil thread")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.store.Put(thread)
	return thread.ID, nil
}

// Resume loads a thread from storage and returns it ready for continued execution.
// The returned thread is the live object from the store, not a copy.
func (cm *CheckpointManager) Resume(_ context.Context, threadID string) (*Thread, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	thread, ok := cm.store.Get(threadID)
	if !ok {
		return nil, fmt.Errorf("session: thread %q not found", threadID)
	}
	return thread, nil
}

// Pause saves the thread state and marks it as paused by appending a checkpoint
// event with StatusPaused metadata. This is the breakpoint between tool selection
// and tool execution. Returns the thread ID for later resumption.
func (cm *CheckpointManager) Pause(ctx context.Context, thread *Thread, reason string) (string, error) {
	if thread == nil {
		return "", errors.New("session: cannot pause nil thread")
	}

	evt, err := NewEvent(EventCheckpoint, "paused")
	if err != nil {
		return "", fmt.Errorf("session: create checkpoint event: %w", err)
	}
	evt.Metadata[checkpointStatusKey] = string(StatusPaused)
	if reason != "" {
		evt.Metadata[checkpointReasonKey] = reason
	}

	thread.Append(evt)
	return cm.Save(ctx, thread)
}

// Status returns the current lifecycle state of a thread by inspecting its
// event history. The status is determined by examining the last event:
//   - If the last event is a checkpoint with status metadata, that status is used.
//   - If events exist after the last checkpoint (i.e., work resumed), the thread
//     is Running regardless of the checkpoint status.
//   - If the thread is empty or has no checkpoints, it is Running.
//
// Terminal states (Completed, Failed) are sticky: once a thread is marked
// completed or failed, appending non-checkpoint events does not change the status.
func (cm *CheckpointManager) Status(_ context.Context, threadID string) (ThreadStatus, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	thread, ok := cm.store.Get(threadID)
	if !ok {
		return "", fmt.Errorf("session: thread %q not found", threadID)
	}

	return threadStatus(thread), nil
}

// threadStatus derives the ThreadStatus from a thread's event history.
func threadStatus(thread *Thread) ThreadStatus {
	events := thread.Replay()
	if len(events) == 0 {
		return StatusRunning
	}

	// Find the most recent checkpoint event.
	lastCheckpointIdx := -1
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == EventCheckpoint {
			lastCheckpointIdx = i
			break
		}
	}

	// No checkpoint events at all — thread is running.
	if lastCheckpointIdx < 0 {
		return StatusRunning
	}

	checkpoint := events[lastCheckpointIdx]
	s, ok := checkpoint.Metadata[checkpointStatusKey]
	if !ok {
		return StatusRunning
	}

	status := ThreadStatus(s)

	// Terminal states are sticky — completed/failed threads stay that way.
	if status == StatusCompleted || status == StatusFailed {
		return status
	}

	// For paused status: if there are non-checkpoint events after the
	// pause checkpoint, the thread has resumed and is running.
	if status == StatusPaused && lastCheckpointIdx < len(events)-1 {
		return StatusRunning
	}

	return status
}

// ListPaused returns all threads currently in the paused state.
func (cm *CheckpointManager) ListPaused(_ context.Context) ([]*Thread, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var paused []*Thread
	for _, thread := range cm.store.All() {
		if threadStatus(thread) == StatusPaused {
			paused = append(paused, thread)
		}
	}
	return paused, nil
}

// Complete marks a thread as completed by appending a checkpoint event with
// StatusCompleted metadata.
func (cm *CheckpointManager) Complete(ctx context.Context, thread *Thread) (string, error) {
	if thread == nil {
		return "", errors.New("session: cannot complete nil thread")
	}

	evt, err := NewEvent(EventCheckpoint, "completed")
	if err != nil {
		return "", fmt.Errorf("session: create checkpoint event: %w", err)
	}
	evt.Metadata[checkpointStatusKey] = string(StatusCompleted)

	thread.Append(evt)
	return cm.Save(ctx, thread)
}

// Fail marks a thread as failed by appending a checkpoint event with
// StatusFailed metadata and the error reason.
func (cm *CheckpointManager) Fail(ctx context.Context, thread *Thread, reason string) (string, error) {
	if thread == nil {
		return "", errors.New("session: cannot fail nil thread")
	}

	evt, err := NewEvent(EventCheckpoint, "failed")
	if err != nil {
		return "", fmt.Errorf("session: create checkpoint event: %w", err)
	}
	evt.Metadata[checkpointStatusKey] = string(StatusFailed)
	if reason != "" {
		evt.Metadata[checkpointReasonKey] = reason
	}

	thread.Append(evt)
	return cm.Save(ctx, thread)
}
