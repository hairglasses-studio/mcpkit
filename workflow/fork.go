package workflow

import (
	"context"
	"fmt"
	"maps"
	"sync"
)

// MergeFunc combines results from parallel branches into a single State.
// branchStates is keyed by branch name.
type MergeFunc func(base State, branchStates map[string]State) (State, error)

// AddForkNode adds a fork node that runs branches in parallel via goroutines,
// then merges results using the merge function. The fork node appears as a
// single node in the graph; branches are internal and not visible as graph nodes.
func (g *Graph) AddForkNode(name string, branches map[string]NodeFunc, merge MergeFunc, opts ...NodeOption) error {
	if name == "" {
		return fmt.Errorf("workflow: fork node name cannot be empty")
	}
	if name == EndNode {
		return fmt.Errorf("workflow: %q is reserved", EndNode)
	}
	if len(branches) == 0 {
		return fmt.Errorf("workflow: fork node %q must have at least one branch", name)
	}
	if merge == nil {
		return fmt.Errorf("workflow: fork node %q merge function cannot be nil", name)
	}
	for bname, fn := range branches {
		if fn == nil {
			return fmt.Errorf("workflow: fork node %q branch %q handler cannot be nil", name, bname)
		}
	}
	if _, exists := g.nodes[name]; exists {
		return fmt.Errorf("workflow: duplicate node %q", name)
	}

	var cfg nodeConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	// Snapshot the branches map so the closure captures a stable copy.
	branchCopy := make(map[string]NodeFunc, len(branches))
	maps.Copy(branchCopy, branches)

	// Create a NodeFunc that runs all branches in parallel.
	forkFn := func(ctx context.Context, state State) (State, error) {
		var mu sync.Mutex
		results := make(map[string]State, len(branchCopy))
		var firstErr error

		var wg sync.WaitGroup
		for bname, fn := range branchCopy {
			wg.Add(1)
			go func(branchName string, branchFn NodeFunc) {
				defer wg.Done()
				branchState := state.Clone()
				branchState.NodeName = name + "/" + branchName

				result, err := branchFn(ctx, branchState)

				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("branch %q: %w", branchName, err)
					}
					return
				}
				results[branchName] = result
			}(bname, fn)
		}
		wg.Wait()

		if firstErr != nil {
			return state, firstErr
		}

		return merge(state, results)
	}

	g.nodes[name] = &node{name: name, fn: forkFn, config: cfg}
	return nil
}

// MergeAll is a convenience MergeFunc that combines all branch Data and
// Metadata maps into the base state. Later branches overwrite earlier ones
// on key conflicts (map iteration order is undefined).
func MergeAll(base State, branches map[string]State) (State, error) {
	result := base.Clone()
	for _, bs := range branches {
		maps.Copy(result.Data, bs.Data)
		maps.Copy(result.Metadata, bs.Metadata)
	}
	return result, nil
}

// MergeKeyed is a convenience MergeFunc that stores each branch's Data map
// under the branch name key in the merged state's Data.
func MergeKeyed(base State, branches map[string]State) (State, error) {
	result := base.Clone()
	for bname, bs := range branches {
		result.Data[bname] = bs.Data
	}
	return result, nil
}
