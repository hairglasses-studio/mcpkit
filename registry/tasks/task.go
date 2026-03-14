package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TaskEntry wraps an mcp.Task with internal management fields.
type TaskEntry struct {
	mu        sync.RWMutex
	Task      mcp.Task
	Result    *mcp.CallToolResult
	ExpiresAt time.Time
	CancelFn  func()
}

// Update modifies the task's status and message atomically.
func (e *TaskEntry) Update(status mcp.TaskStatus, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Task.Status = status
	e.Task.StatusMessage = message
	e.Task.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// SetResult stores the completed result.
func (e *TaskEntry) SetResult(result *mcp.CallToolResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Result = result
}

// Snapshot returns a copy of the current task state.
func (e *TaskEntry) Snapshot() mcp.Task {
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
