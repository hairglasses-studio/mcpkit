package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrDelegationInvalidNode is returned when a delegation node is missing
	// an ID or stage.
	ErrDelegationInvalidNode = errors.New("orchestrator: invalid delegation node")
	// ErrDelegationMaxDepth is returned when a delegation tree exceeds MaxDepth.
	ErrDelegationMaxDepth = errors.New("orchestrator: delegation max depth exceeded")
)

// DelegationAggregator combines a node's own output with child results.
type DelegationAggregator func(self *StageOutput, children []*DelegationResult) (*StageOutput, error)

// DelegationNode is one manager or worker in a delegation tree.
type DelegationNode struct {
	ID         string
	Stage      StageFunc
	Children   []DelegationNode
	Aggregator DelegationAggregator
}

// DelegationConfig configures hierarchical delegation.
type DelegationConfig struct {
	Timeout          time.Duration
	MaxDepth         int
	ChildConcurrency int
}

// DelegationResult is the nested output from a delegation tree.
type DelegationResult struct {
	ID          string              `json:"id"`
	SelfOutput  *StageOutput        `json:"self_output,omitempty"`
	Output      *StageOutput        `json:"output,omitempty"`
	Children    []*DelegationResult `json:"children,omitempty"`
	Error       string              `json:"error,omitempty"`
	Duration    time.Duration       `json:"duration_ns"`
	Depth       int                 `json:"depth"`
	ChildCount  int                 `json:"child_count"`
	FailedCount int                 `json:"failed_count"`
}

// HierarchicalDelegation executes a manager/worker tree. Each child receives
// the merged data from its parent output and a Previous pointer to that output.
func HierarchicalDelegation(ctx context.Context, root DelegationNode, input StageInput, config ...DelegationConfig) (*DelegationResult, error) {
	cfg := delegationConfig(config...)
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	return executeDelegationNode(ctx, root, input, cfg, 0)
}

func executeDelegationNode(ctx context.Context, node DelegationNode, input StageInput, cfg DelegationConfig, depth int) (*DelegationResult, error) {
	if node.ID == "" || node.Stage == nil {
		return nil, ErrDelegationInvalidNode
	}
	if depth > cfg.MaxDepth {
		return nil, fmt.Errorf("%w: %s", ErrDelegationMaxDepth, node.ID)
	}

	start := time.Now()
	result := &DelegationResult{ID: node.ID, Depth: depth, ChildCount: len(node.Children)}

	stageStart := time.Now()
	self, err := node.Stage(ctx, input)
	if self != nil {
		self.Duration = time.Since(stageStart)
	}
	result.SelfOutput = self
	result.Output = self
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result, err
	}

	childInput := StageInput{
		Data:     mergeData(input.Data, self),
		Previous: self,
		Metadata: input.Metadata,
	}
	children, childErr := executeDelegationChildren(ctx, node.Children, childInput, cfg, depth+1)
	result.Children = children
	result.FailedCount = countFailedDelegation(children)

	if node.Aggregator != nil {
		aggregated, aggErr := node.Aggregator(self, children)
		if aggregated != nil {
			result.Output = aggregated
		}
		if aggErr != nil && childErr == nil {
			childErr = aggErr
		}
		if aggErr != nil {
			result.Error = aggErr.Error()
		}
	}
	if childErr != nil && result.Error == "" {
		result.Error = childErr.Error()
	}
	result.Duration = time.Since(start)
	return result, childErr
}

func executeDelegationChildren(ctx context.Context, children []DelegationNode, input StageInput, cfg DelegationConfig, depth int) ([]*DelegationResult, error) {
	if len(children) == 0 {
		return nil, nil
	}
	results := make([]*DelegationResult, len(children))
	sem := make(chan struct{}, maxConcurrency(cfg.ChildConcurrency, len(children)))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, child := range children {
		wg.Add(1)
		go func(idx int, child DelegationNode) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				results[idx] = &DelegationResult{ID: child.ID, Error: ctx.Err().Error(), Depth: depth}
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				return
			}

			childResult, err := executeDelegationNode(ctx, child, input, cfg, depth)
			mu.Lock()
			results[idx] = childResult
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(i, child)
	}
	wg.Wait()
	return results, firstErr
}

func delegationConfig(config ...DelegationConfig) DelegationConfig {
	cfg := DelegationConfig{MaxDepth: 32}
	if len(config) > 0 {
		cfg = config[0]
		if cfg.MaxDepth == 0 {
			cfg.MaxDepth = 32
		}
	}
	return cfg
}

func countFailedDelegation(children []*DelegationResult) int {
	failed := 0
	for _, child := range children {
		if child != nil && child.Error != "" {
			failed++
		}
	}
	return failed
}
