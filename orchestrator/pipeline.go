package orchestrator

import (
	"context"
	"maps"
	"time"
)

// PipelineConfig configures sequential execution.
type PipelineConfig struct {
	Timeout      time.Duration
	OnStageError string // "stop" (default), "skip", "continue"
}

// Pipeline executes stages sequentially, passing each output as input to the next.
func Pipeline(ctx context.Context, stages []StageFunc, input StageInput, config ...PipelineConfig) (*OrchestratorResult, error) {
	cfg := PipelineConfig{OnStageError: "stop"}
	if len(config) > 0 {
		cfg = config[0]
		if cfg.OnStageError == "" {
			cfg.OnStageError = "stop"
		}
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	start := time.Now()
	outputs := make([]*StageOutput, 0, len(stages))
	current := input

	for _, stage := range stages {
		stageStart := time.Now()
		out, err := stage(ctx, current)
		elapsed := time.Since(stageStart)

		if err != nil {
			errOutput := &StageOutput{
				Status:   "error",
				Error:    err.Error(),
				Duration: elapsed,
			}
			outputs = append(outputs, errOutput)

			switch cfg.OnStageError {
			case "stop":
				return &OrchestratorResult{
					Pattern:    "pipeline",
					Outputs:    outputs,
					Duration:   time.Since(start),
					StageCount: len(outputs),
				}, err
			case "skip":
				// Don't update current; continue to next stage with same input.
				continue
			case "continue":
				// Pass the error output as Previous to next stage.
				current.Previous = errOutput
				continue
			}
		}

		if out != nil {
			out.Duration = elapsed
		}
		outputs = append(outputs, out)

		// Pass output as Previous to next stage, and merge Data.
		current = StageInput{
			Data:     mergeData(current.Data, out),
			Previous: out,
			Metadata: current.Metadata,
		}
	}

	return &OrchestratorResult{
		Pattern:    "pipeline",
		Outputs:    outputs,
		Duration:   time.Since(start),
		StageCount: len(outputs),
	}, nil
}

func mergeData(base map[string]any, out *StageOutput) map[string]any {
	merged := make(map[string]any)
	maps.Copy(merged, base)
	if out != nil {
		maps.Copy(merged, out.Data)
	}
	return merged
}
