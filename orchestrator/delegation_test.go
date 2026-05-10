package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHierarchicalDelegationAggregatesChildren(t *testing.T) {
	t.Parallel()

	manager := DelegationNode{
		ID: "manager",
		Stage: func(_ context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"plan": "ship"}}, nil
		},
		Children: []DelegationNode{
			{
				ID: "worker-a",
				Stage: func(_ context.Context, input StageInput) (*StageOutput, error) {
					if input.Data["plan"] != "ship" {
						t.Fatalf("child did not receive parent data: %+v", input.Data)
					}
					return &StageOutput{Status: "ok", Data: map[string]any{"a": 1}}, nil
				},
			},
			{
				ID: "worker-b",
				Stage: func(_ context.Context, input StageInput) (*StageOutput, error) {
					if input.Previous == nil || input.Previous.Data["plan"] != "ship" {
						t.Fatalf("child did not receive previous parent output: %+v", input.Previous)
					}
					return &StageOutput{Status: "ok", Data: map[string]any{"b": 2}}, nil
				},
			},
		},
		Aggregator: func(self *StageOutput, children []*DelegationResult) (*StageOutput, error) {
			total := 0
			for _, child := range children {
				for _, value := range child.Output.Data {
					if n, ok := value.(int); ok {
						total += n
					}
				}
			}
			return &StageOutput{Status: "ok", Data: map[string]any{"plan": self.Data["plan"], "total": total}}, nil
		},
	}

	result, err := HierarchicalDelegation(context.Background(), manager, StageInput{Data: map[string]any{"request": "build"}})
	if err != nil {
		t.Fatalf("HierarchicalDelegation: %v", err)
	}
	if result.ID != "manager" || result.ChildCount != 2 || len(result.Children) != 2 {
		t.Fatalf("unexpected result shape: %+v", result)
	}
	if result.Output.Data["total"] != 3 {
		t.Fatalf("aggregated total = %v", result.Output.Data["total"])
	}
}

func TestHierarchicalDelegationNestedTree(t *testing.T) {
	t.Parallel()

	leaf := DelegationNode{
		ID: "leaf",
		Stage: func(_ context.Context, input StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"depth": input.Data["depth"]}}, nil
		},
	}
	root := DelegationNode{
		ID: "root",
		Stage: func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"depth": 1}}, nil
		},
		Children: []DelegationNode{{
			ID: "middle",
			Stage: func(_ context.Context, input StageInput) (*StageOutput, error) {
				return &StageOutput{Status: "ok", Data: map[string]any{"depth": input.Data["depth"].(int) + 1}}, nil
			},
			Children: []DelegationNode{leaf},
		}},
	}

	result, err := HierarchicalDelegation(context.Background(), root, StageInput{})
	if err != nil {
		t.Fatalf("HierarchicalDelegation: %v", err)
	}
	if result.Children[0].Children[0].Depth != 2 {
		t.Fatalf("leaf depth = %d", result.Children[0].Children[0].Depth)
	}
}

func TestHierarchicalDelegationValidation(t *testing.T) {
	t.Parallel()

	_, err := HierarchicalDelegation(context.Background(), DelegationNode{}, StageInput{})
	if !errors.Is(err, ErrDelegationInvalidNode) {
		t.Fatalf("invalid node error = %v", err)
	}
}

func TestHierarchicalDelegationMaxDepth(t *testing.T) {
	t.Parallel()

	root := DelegationNode{
		ID: "root",
		Stage: func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok"}, nil
		},
		Children: []DelegationNode{{
			ID: "child",
			Stage: func(context.Context, StageInput) (*StageOutput, error) {
				return &StageOutput{Status: "ok"}, nil
			},
		}},
	}
	result, err := HierarchicalDelegation(context.Background(), root, StageInput{}, DelegationConfig{MaxDepth: 0})
	if err != nil {
		t.Fatalf("default max depth should allow child: %v", err)
	}
	if len(result.Children) != 1 {
		t.Fatalf("children len = %d", len(result.Children))
	}

	_, err = HierarchicalDelegation(context.Background(), root, StageInput{}, DelegationConfig{MaxDepth: -1})
	if !errors.Is(err, ErrDelegationMaxDepth) {
		t.Fatalf("max depth error = %v", err)
	}
}

func TestHierarchicalDelegationPropagatesChildError(t *testing.T) {
	t.Parallel()

	childErr := errors.New("child failed")
	root := DelegationNode{
		ID: "root",
		Stage: func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok"}, nil
		},
		Children: []DelegationNode{{
			ID: "child",
			Stage: func(context.Context, StageInput) (*StageOutput, error) {
				return nil, childErr
			},
		}},
	}
	result, err := HierarchicalDelegation(context.Background(), root, StageInput{})
	if !errors.Is(err, childErr) {
		t.Fatalf("error = %v, want %v", err, childErr)
	}
	if result.FailedCount != 1 || result.Children[0].Error != childErr.Error() {
		t.Fatalf("unexpected failed child data: %+v", result)
	}
}

func TestHierarchicalDelegationChildConcurrency(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	makeChild := func(id string) DelegationNode {
		return DelegationNode{
			ID: id,
			Stage: func(ctx context.Context, input StageInput) (*StageOutput, error) {
				started <- struct{}{}
				select {
				case <-release:
					return &StageOutput{Status: "ok"}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
	}
	root := DelegationNode{
		ID: "root",
		Stage: func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok"}, nil
		},
		Children: []DelegationNode{makeChild("a"), makeChild("b")},
	}

	done := make(chan struct{})
	go func() {
		_, _ = HierarchicalDelegation(context.Background(), root, StageInput{}, DelegationConfig{ChildConcurrency: 1})
		close(done)
	}()

	<-started
	select {
	case <-started:
		t.Fatal("second child started despite ChildConcurrency=1")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delegation did not finish")
	}
}
