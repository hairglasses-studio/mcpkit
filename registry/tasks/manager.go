package tasks

import (
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// DefaultTTL is the default time-to-live for tasks (10 minutes).
const DefaultTTL = 10 * time.Minute

// Manager manages the lifecycle of tasks.
type Manager interface {
	// Create creates a new task with the given TTL. If ttl is 0, DefaultTTL is used.
	Create(ttl time.Duration) *TaskEntry
	// Get returns a task by ID, or nil if not found.
	Get(taskID string) *TaskEntry
	// List returns all non-expired tasks.
	List() []mcp.Task
	// Cancel cancels a task by ID. Returns an error if not found or already terminal.
	Cancel(taskID string) error
	// Cleanup removes expired tasks. Call periodically or let middleware handle it.
	Cleanup()
	// Count returns the number of active (non-expired) tasks.
	Count() int
}

// InMemoryManager is a thread-safe, in-memory task manager.
type InMemoryManager struct {
	mu    sync.RWMutex
	tasks map[string]*TaskEntry
}

// NewManager creates a new in-memory task manager.
func NewManager() *InMemoryManager {
	return &InMemoryManager{
		tasks: make(map[string]*TaskEntry),
	}
}

func (m *InMemoryManager) Create(ttl time.Duration) *TaskEntry {
	if ttl == 0 {
		ttl = DefaultTTL
	}

	id := GenerateID()
	ttlMs := ttl.Milliseconds()
	task := mcp.NewTask(id, mcp.WithTaskTTL(ttlMs))

	entry := &TaskEntry{
		Task:      task,
		ExpiresAt: time.Now().Add(ttl),
	}

	m.mu.Lock()
	m.tasks[id] = entry
	m.mu.Unlock()

	return entry
}

func (m *InMemoryManager) Get(taskID string) *TaskEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry := m.tasks[taskID]
	if entry != nil && entry.IsExpired() {
		return nil
	}
	return entry
}

func (m *InMemoryManager) List() []mcp.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []mcp.Task
	for _, entry := range m.tasks {
		if !entry.IsExpired() {
			result = append(result, entry.Snapshot())
		}
	}
	return result
}

func (m *InMemoryManager) Cancel(taskID string) error {
	m.mu.RLock()
	entry := m.tasks[taskID]
	m.mu.RUnlock()

	if entry == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	snapshot := entry.Snapshot()
	if snapshot.Status.IsTerminal() {
		return fmt.Errorf("task %s is already in terminal state: %s", taskID, snapshot.Status)
	}

	entry.Update(mcp.TaskStatusCancelled, "cancelled by client")
	if entry.CancelFn != nil {
		entry.CancelFn()
	}
	return nil
}

func (m *InMemoryManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.tasks {
		if entry.IsExpired() {
			delete(m.tasks, id)
		}
	}
}

func (m *InMemoryManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, entry := range m.tasks {
		if !entry.IsExpired() {
			count++
		}
	}
	return count
}
