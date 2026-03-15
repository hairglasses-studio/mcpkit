//go:build !official_sdk

package ralph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/workflow"
)

// WorkflowConfig configures a WorkflowLoop execution.
type WorkflowConfig struct {
	// Engine is the workflow engine to execute.
	Engine *workflow.Engine
	// RunID identifies this execution for checkpointing.
	RunID string
	// InitialState is the starting state for the workflow.
	InitialState workflow.State
	// Hooks provides lifecycle callbacks.
	Hooks Hooks
	// MaxDuration is the maximum total duration (0 = no limit).
	MaxDuration time.Duration
}

// WorkflowLoop bridges ralph's lifecycle/hooks with the workflow engine.
// It runs the workflow engine and translates its results into ralph Progress.
type WorkflowLoop struct {
	config   WorkflowConfig
	mu       sync.Mutex
	progress Progress
	stopCh   chan struct{}
	stopped  bool
}

// NewWorkflowLoop creates a WorkflowLoop from the given config.
func NewWorkflowLoop(config WorkflowConfig) (*WorkflowLoop, error) {
	if config.Engine == nil {
		return nil, fmt.Errorf("ralph: workflow engine is required")
	}
	if config.RunID == "" {
		config.RunID = fmt.Sprintf("wf-%d", time.Now().UnixNano())
	}
	return &WorkflowLoop{
		config: config,
		stopCh: make(chan struct{}),
	}, nil
}

// Run executes the workflow engine and returns when complete, failed, or stopped.
func (wl *WorkflowLoop) Run(ctx context.Context) error {
	wl.mu.Lock()
	wl.progress = Progress{
		Status:    StatusRunning,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	wl.mu.Unlock()

	wl.config.Hooks.callIterationStart(1)

	// Apply max duration if configured.
	if wl.config.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, wl.config.MaxDuration)
		defer cancel()
	}

	// Create a cancellable context for stop signal.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// Watch for stop signal.
	go func() {
		select {
		case <-wl.stopCh:
			runCancel()
		case <-runCtx.Done():
		}
	}()

	// Execute the workflow.
	result, err := wl.config.Engine.Run(runCtx, wl.config.RunID, wl.config.InitialState)
	if err != nil {
		wl.setStatus(StatusFailed)
		wl.config.Hooks.callError(1, err)
		return fmt.Errorf("ralph: workflow execution: %w", err)
	}

	// Map workflow result to ralph status.
	entry := IterationLog{
		Iteration: 1,
		Result:    fmt.Sprintf("workflow %s in %d steps (%v)", result.Status, result.Steps, result.Duration),
		Timestamp: time.Now(),
	}

	switch result.Status {
	case workflow.RunStatusCompleted:
		wl.setStatus(StatusCompleted)
	case workflow.RunStatusFailed:
		wl.setStatus(StatusFailed)
		entry.Result = fmt.Sprintf("workflow failed: %s", result.Error)
		wl.config.Hooks.callError(1, fmt.Errorf("%s", result.Error)) //nolint:goerr113
	case workflow.RunStatusStopped:
		wl.setStatus(StatusStopped)
	}

	wl.mu.Lock()
	wl.progress.Log = append(wl.progress.Log, entry)
	wl.progress.Iteration = result.Steps
	wl.progress.UpdatedAt = time.Now()
	wl.mu.Unlock()

	wl.config.Hooks.callIterationEnd(entry)

	if result.Status == workflow.RunStatusFailed {
		return fmt.Errorf("ralph: workflow failed: %s", result.Error)
	}

	return nil
}

// Status returns the current progress snapshot.
func (wl *WorkflowLoop) Status() Progress {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	return wl.progress
}

// Stop signals the workflow to stop gracefully.
func (wl *WorkflowLoop) Stop() {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	if !wl.stopped {
		wl.stopped = true
		close(wl.stopCh)
	}
}

func (wl *WorkflowLoop) setStatus(s Status) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	wl.progress.Status = s
	wl.progress.UpdatedAt = time.Now()
}
