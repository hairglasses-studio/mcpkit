package workflow

import (
	"context"
	"fmt"
	"time"
)

// RunStatus represents the final status of a workflow run.
type RunStatus string

const (
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusStopped   RunStatus = "stopped"
)

// RunResult is the outcome of a workflow execution.
type RunResult struct {
	RunID      string        `json:"run_id"`
	Status     RunStatus     `json:"status"`
	FinalState State         `json:"final_state"`
	Steps      int           `json:"steps"`
	Duration   time.Duration `json:"duration_ns"`
	Error      string        `json:"error,omitempty"`
}

// EngineConfig configures the workflow engine.
type EngineConfig struct {
	MaxSteps           int
	DefaultNodeTimeout time.Duration
	CheckpointStore    CheckpointStore
	Hooks              Hooks
	// NodeMiddleware is applied to every node function before execution.
	NodeMiddleware []NodeMiddleware
}

// Engine executes workflow graphs step-by-step.
type Engine struct {
	graph  *Graph
	config EngineConfig
}

// NewEngine creates a new engine for the given graph.
func NewEngine(graph *Graph, config ...EngineConfig) (*Engine, error) {
	if err := graph.Validate(); err != nil {
		return nil, err
	}
	var cfg EngineConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 1000
	}
	if cfg.DefaultNodeTimeout <= 0 {
		cfg.DefaultNodeTimeout = 30 * time.Second
	}
	return &Engine{graph: graph, config: cfg}, nil
}

// Run executes the graph from the start node with the given initial state.
func (e *Engine) Run(ctx context.Context, runID string, initial State) (*RunResult, error) {
	start := time.Now()
	state := initial.Clone()
	currentNode := e.graph.start

	for step := 0; step < e.config.MaxSteps; step++ {
		if ctx.Err() != nil {
			return &RunResult{
				RunID:      runID,
				Status:     RunStatusStopped,
				FinalState: state,
				Steps:      step,
				Duration:   time.Since(start),
				Error:      ctx.Err().Error(),
			}, nil
		}

		n, ok := e.graph.nodes[currentNode]
		if !ok {
			return nil, fmt.Errorf("workflow: node %q not found during execution", currentNode)
		}

		state.Step = step
		state.NodeName = currentNode
		state.UpdatedAt = time.Now()

		e.config.Hooks.callNodeStart(currentNode, state)

		// Execute the node with timeout
		timeout := n.config.timeout
		if timeout <= 0 {
			timeout = e.config.DefaultNodeTimeout
		}
		fn := n.fn
		if len(e.config.NodeMiddleware) > 0 {
			fn = WrapNodeFunc(fn, currentNode, e.config.NodeMiddleware...)
		}
		nodeCtx, cancel := context.WithTimeout(ctx, timeout)
		newState, err := fn(nodeCtx, state.Clone())
		cancel()

		if err != nil {
			e.config.Hooks.callNodeError(currentNode, err)
			return &RunResult{
				RunID:      runID,
				Status:     RunStatusFailed,
				FinalState: state,
				Steps:      step + 1,
				Duration:   time.Since(start),
				Error:      fmt.Sprintf("node %q: %v", currentNode, err),
			}, nil
		}

		state = newState
		e.config.Hooks.callNodeEnd(currentNode, state)

		// Checkpoint after node completion
		if e.config.CheckpointStore != nil {
			cp := Checkpoint{
				RunID:       runID,
				State:       state,
				CurrentNode: currentNode,
				Step:        step + 1,
				SavedAt:     time.Now(),
			}
			if saveErr := e.config.CheckpointStore.Save(ctx, cp); saveErr != nil {
				return nil, fmt.Errorf("workflow: checkpoint save: %w", saveErr)
			}
			e.config.Hooks.callCheckpoint(cp)
		}

		// Determine next node
		nextNode, err := e.resolveNext(n, state)
		if err != nil {
			return nil, err
		}

		if nextNode == EndNode {
			result := &RunResult{
				RunID:      runID,
				Status:     RunStatusCompleted,
				FinalState: state,
				Steps:      step + 1,
				Duration:   time.Since(start),
			}
			// Clean up checkpoint on completion
			if e.config.CheckpointStore != nil {
				_ = e.config.CheckpointStore.Delete(ctx, runID)
			}
			return result, nil
		}

		// Cycle detection hook
		if nextNode == currentNode {
			e.config.Hooks.callCycleDetected(currentNode, step+1)
		}

		currentNode = nextNode
	}

	// MaxSteps exceeded
	return &RunResult{
		RunID:      runID,
		Status:     RunStatusFailed,
		FinalState: state,
		Steps:      e.config.MaxSteps,
		Duration:   time.Since(start),
		Error:      fmt.Sprintf("max steps (%d) exceeded", e.config.MaxSteps),
	}, nil
}

// Resume resumes a workflow from the last checkpoint.
func (e *Engine) Resume(ctx context.Context, runID string) (*RunResult, error) {
	if e.config.CheckpointStore == nil {
		return nil, fmt.Errorf("workflow: no checkpoint store configured")
	}
	cp, found, err := e.config.CheckpointStore.Load(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("workflow: load checkpoint: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("workflow: no checkpoint found for run %q", runID)
	}

	// Determine next node from the checkpoint's current node
	n, ok := e.graph.nodes[cp.CurrentNode]
	if !ok {
		return nil, fmt.Errorf("workflow: checkpoint node %q not found in graph", cp.CurrentNode)
	}
	nextNode, err := e.resolveNext(n, cp.State)
	if err != nil {
		return nil, err
	}
	if nextNode == EndNode {
		_ = e.config.CheckpointStore.Delete(ctx, runID)
		return &RunResult{
			RunID:      runID,
			Status:     RunStatusCompleted,
			FinalState: cp.State,
			Steps:      cp.Step,
		}, nil
	}

	// Create a modified engine config with reduced remaining steps
	remaining := e.config.MaxSteps - cp.Step
	if remaining <= 0 {
		return &RunResult{
			RunID:      runID,
			Status:     RunStatusFailed,
			FinalState: cp.State,
			Steps:      cp.Step,
			Error:      fmt.Sprintf("max steps (%d) already reached at checkpoint", e.config.MaxSteps),
		}, nil
	}

	// Run from the next node with the checkpoint state
	// We re-use Run by temporarily modifying graph start, but cleaner to inline the loop
	start := time.Now()
	state := cp.State
	currentNode := nextNode

	for step := cp.Step; step < e.config.MaxSteps; step++ {
		if ctx.Err() != nil {
			return &RunResult{
				RunID:      runID,
				Status:     RunStatusStopped,
				FinalState: state,
				Steps:      step,
				Duration:   time.Since(start),
				Error:      ctx.Err().Error(),
			}, nil
		}

		nd, ok := e.graph.nodes[currentNode]
		if !ok {
			return nil, fmt.Errorf("workflow: node %q not found during execution", currentNode)
		}

		state.Step = step
		state.NodeName = currentNode
		state.UpdatedAt = time.Now()

		e.config.Hooks.callNodeStart(currentNode, state)

		timeout := nd.config.timeout
		if timeout <= 0 {
			timeout = e.config.DefaultNodeTimeout
		}
		fn := nd.fn
		if len(e.config.NodeMiddleware) > 0 {
			fn = WrapNodeFunc(fn, currentNode, e.config.NodeMiddleware...)
		}
		nodeCtx, cancel := context.WithTimeout(ctx, timeout)
		newState, nodeErr := fn(nodeCtx, state.Clone())
		cancel()

		if nodeErr != nil {
			e.config.Hooks.callNodeError(currentNode, nodeErr)
			return &RunResult{
				RunID:      runID,
				Status:     RunStatusFailed,
				FinalState: state,
				Steps:      step + 1,
				Duration:   time.Since(start),
				Error:      fmt.Sprintf("node %q: %v", currentNode, nodeErr),
			}, nil
		}

		state = newState
		e.config.Hooks.callNodeEnd(currentNode, state)

		if e.config.CheckpointStore != nil {
			cpNew := Checkpoint{
				RunID:       runID,
				State:       state,
				CurrentNode: currentNode,
				Step:        step + 1,
				SavedAt:     time.Now(),
			}
			if saveErr := e.config.CheckpointStore.Save(ctx, cpNew); saveErr != nil {
				return nil, fmt.Errorf("workflow: checkpoint save: %w", saveErr)
			}
			e.config.Hooks.callCheckpoint(cpNew)
		}

		next, resolveErr := e.resolveNext(nd, state)
		if resolveErr != nil {
			return nil, resolveErr
		}

		if next == EndNode {
			result := &RunResult{
				RunID:      runID,
				Status:     RunStatusCompleted,
				FinalState: state,
				Steps:      step + 1,
				Duration:   time.Since(start),
			}
			if e.config.CheckpointStore != nil {
				_ = e.config.CheckpointStore.Delete(ctx, runID)
			}
			return result, nil
		}

		if next == currentNode {
			e.config.Hooks.callCycleDetected(currentNode, step+1)
		}

		currentNode = next
	}

	return &RunResult{
		RunID:      runID,
		Status:     RunStatusFailed,
		FinalState: state,
		Steps:      e.config.MaxSteps,
		Duration:   time.Since(start),
		Error:      fmt.Sprintf("max steps (%d) exceeded", e.config.MaxSteps),
	}, nil
}

// resolveNext determines the next node from edges.
func (e *Engine) resolveNext(n *node, state State) (string, error) {
	for _, edge := range n.edges {
		if edge.condition != nil {
			target := edge.condition(state)
			if target == EndNode {
				return EndNode, nil
			}
			if _, ok := e.graph.nodes[target]; !ok {
				return "", fmt.Errorf("workflow: conditional edge from %q returned unknown node %q", n.name, target)
			}
			return target, nil
		}
		// Unconditional edge
		return edge.to, nil
	}
	return "", fmt.Errorf("workflow: node %q has no edges", n.name)
}
