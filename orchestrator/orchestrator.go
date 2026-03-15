package orchestrator

import (
	"context"
	"time"
)

// StageFunc executes a single orchestration stage.
type StageFunc func(ctx context.Context, input StageInput) (*StageOutput, error)

// StageInput is the input to a stage.
type StageInput struct {
	Data     map[string]any
	Previous *StageOutput  // nil for first stage
	Metadata map[string]string
}

// StageOutput is the output of a stage.
type StageOutput struct {
	Data     map[string]any    `json:"data"`
	Status   string            `json:"status"` // ok, error, skip
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration_ns"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OrchestratorResult aggregates outputs from an orchestration pattern.
type OrchestratorResult struct {
	Pattern    string         `json:"pattern"`
	Outputs    []*StageOutput `json:"outputs"`
	Duration   time.Duration  `json:"duration_ns"`
	StageCount int            `json:"stage_count"`
}
