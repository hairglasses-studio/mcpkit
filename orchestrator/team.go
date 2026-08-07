package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// TaskStatus represents the current state of a task in a team.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// Task represents a single unit of work assigned to a team worker.
type Task struct {
	ID          string
	Title       string
	Description string
	Status      TaskStatus
	Result      string
	Error       string
	StartedAt   time.Time
	EndedAt     time.Time
	WorkerID    string
}

// Team manages a group of agents working together on a set of tasks.
type Team struct {
	Name           string
	Planner        sampling.SamplingClient
	WorkerRegistry *registry.ToolRegistry
	MaxConcurrency int
	Tasks          []*Task

	// Executor performs one task and returns its result. When nil, runTask
	// FAILS the task with an explicit not-implemented result — it must never
	// fabricate success (the previous sleep→"Success" stub was flagged
	// delete-or-implement by the 2026-08-06 a2a-stack reconciliation; a fake
	// green here is the exact false-positive class the estate's
	// false-green-taxonomy.md catalogues).
	Executor func(ctx context.Context, task *Task) (result string, err error)

	mu sync.Mutex
}

// NewTeam creates a new orchestrator team.
func NewTeam(name string, planner sampling.SamplingClient, workers *registry.ToolRegistry) *Team {
	return &Team{
		Name:           name,
		Planner:        planner,
		WorkerRegistry: workers,
		MaxConcurrency: 2,
		Tasks:          []*Task{},
	}
}

// AddTask adds a task to the team's queue.
func (t *Team) AddTask(title, description string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("task-%d", len(t.Tasks)+1)
	t.Tasks = append(t.Tasks, &Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      TaskPending,
	})
	return id
}

// Execute drives the team to complete all tasks.
func (t *Team) Execute(ctx context.Context) error {
	for {
		t.mu.Lock()
		allDone := true
		running := 0
		var nextTask *Task

		for _, task := range t.Tasks {
			if task.Status != TaskCompleted && task.Status != TaskFailed {
				allDone = false
			}
			if task.Status == TaskRunning {
				running++
			}
			if task.Status == TaskPending && nextTask == nil {
				nextTask = task
			}
		}

		if allDone {
			t.mu.Unlock()
			return nil
		}

		if nextTask != nil && running < t.MaxConcurrency {
			nextTask.Status = TaskRunning
			nextTask.StartedAt = time.Now()
			go t.runTask(ctx, nextTask)
		}
		t.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func (t *Team) runTask(ctx context.Context, task *Task) {
	slog.Info("team worker starting task", "team", t.Name, "task", task.ID, "title", task.Title)

	if t.Executor == nil {
		// Honest failure beats fake success: no executor configured means
		// this team cannot actually perform work, and reporting anything
		// else would be a fabricated green.
		t.mu.Lock()
		task.Status = TaskFailed
		task.EndedAt = time.Now()
		task.Result = "not implemented: Team.Executor is nil — no worker execution path is configured"
		t.mu.Unlock()
		slog.Warn("team worker failing task: no Executor configured", "team", t.Name, "task", task.ID)
		return
	}

	result, err := t.Executor(ctx, task)
	t.mu.Lock()
	task.EndedAt = time.Now()
	if err != nil {
		task.Status = TaskFailed
		task.Result = "executor error: " + err.Error()
	} else {
		task.Status = TaskCompleted
		task.Result = result
	}
	t.mu.Unlock()

	slog.Info("team worker finished task", "team", t.Name, "task", task.ID, "status", task.Status)
}
