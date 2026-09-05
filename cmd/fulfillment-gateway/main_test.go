package main

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/gateway/fulfillment"
)

func TestCLIE2ERunner(t *testing.T) {
	ctx := context.Background()
	engine := fulfillment.NewPipelineEngine("")
	mm := fulfillment.NewMatrixManager("")

	out := engine.RunE2E(ctx, mm, true, 2, "test-cli-suite")
	if !out.Success {
		t.Fatalf("Expected E2E run to succeed, got: %s", out.Summary)
	}
	if !out.InvariantPassed {
		t.Errorf("Expected invariants to pass")
	}
	if len(out.StepsExecuted) != 2 {
		t.Errorf("Expected 2 steps executed, got %d", len(out.StepsExecuted))
	}
}

func TestDaemonCycleExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	engine := fulfillment.NewPipelineEngine("")
	mm := fulfillment.NewMatrixManager("")

	out := engine.RunE2E(ctx, mm, true, 1, "test-daemon-suite")
	if !out.Success {
		t.Fatalf("Expected initial cycle to succeed")
	}
}

