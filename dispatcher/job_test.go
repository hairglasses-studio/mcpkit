//go:build !official_sdk

package dispatcher

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestPriorityConstants(t *testing.T) {
	t.Parallel()
	if PriorityLow >= PriorityNormal {
		t.Error("expected PriorityLow < PriorityNormal")
	}
	if PriorityNormal >= PriorityHigh {
		t.Error("expected PriorityNormal < PriorityHigh")
	}
	if PriorityHigh >= PriorityCritical {
		t.Error("expected PriorityHigh < PriorityCritical")
	}
}

func TestJob_ZeroValue(t *testing.T) {
	t.Parallel()
	var j Job
	if j.Priority != PriorityLow {
		t.Errorf("expected zero-value Priority to be PriorityLow (0), got %d", j.Priority)
	}
	if j.Group != "" {
		t.Errorf("expected zero-value Group to be empty, got %q", j.Group)
	}
}

func TestJob_Fields(t *testing.T) {
	t.Parallel()
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "my-tool"}}
	ctx := context.Background()
	req := registry.CallToolRequest{}
	handler := func(ctx context.Context, r registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	j := &Job{
		Name:     "my-tool",
		TD:       td,
		Ctx:      ctx,
		Request:  req,
		Handler:  handler,
		Priority: PriorityHigh,
		Group:    "batch",
	}

	if j.Name != "my-tool" {
		t.Errorf("expected Name %q, got %q", "my-tool", j.Name)
	}
	if j.Priority != PriorityHigh {
		t.Errorf("expected Priority PriorityHigh, got %d", j.Priority)
	}
	if j.Group != "batch" {
		t.Errorf("expected Group %q, got %q", "batch", j.Group)
	}
}

func TestJob_ResultChannelBuffered(t *testing.T) {
	t.Parallel()
	// Jobs submitted through the dispatcher have a buffered result channel.
	// Verify the jobResult type carries both fields.
	res := registry.MakeTextResult("hello")
	jr := jobResult{Result: res, Err: nil}
	if jr.Result == nil {
		t.Error("expected non-nil result in jobResult")
	}
	if jr.Err != nil {
		t.Errorf("expected nil error in jobResult, got %v", jr.Err)
	}
}

func TestJobResult_WithError(t *testing.T) {
	t.Parallel()
	errResult := registry.MakeErrorResult("something failed")
	jr := jobResult{Result: errResult, Err: nil}
	if !jr.Result.IsError {
		t.Error("expected IsError to be true for error result")
	}
}
