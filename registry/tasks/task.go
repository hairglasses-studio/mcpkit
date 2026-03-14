package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TaskInfo holds the state of an async task. This is a portable type that
// does not depend on any specific MCP SDK.
type TaskInfo struct {
	TaskId        string
	Status        registry.TaskStatus
	StatusMessage string
	LastUpdatedAt string
}

// IsTerminal returns true if the task is in a terminal state.
func (t TaskInfo) IsTerminal() bool {
	switch t.Status {
	case registry.TaskStatusCompleted, registry.TaskStatusFailed, registry.TaskStatusCancelled:
		return true
	}
	return false
}

// TaskEntry wraps a TaskInfo with internal management fields.
type TaskEntry struct {
	mu        sync.RWMutex
	Task      TaskInfo
	Result    *registry.CallToolResult
	ExpiresAt time.Time
	CancelFn  func()
}

// Update modifies the task's status and message atomically.
func (e *TaskEntry) Update(status registry.TaskStatus, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Task.Status = status
	e.Task.StatusMessage = message
	e.Task.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// SetResult stores the completed result.
func (e *TaskEntry) SetResult(result *registry.CallToolResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Result = result
}

// Snapshot returns a copy of the current task state.
func (e *TaskEntry) Snapshot() TaskInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Task
}

// IsExpired returns true if the task has exceeded its TTL.
func (e *TaskEntry) IsExpired() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// GenerateID creates a unique task ID.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "task_" + hex.EncodeToString(b)
}
