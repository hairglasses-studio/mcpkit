package orchestrator

import (
	"context"
	"sync"
	"time"
)

// FanOutConfig configures parallel execution.
type FanOutConfig struct {
	MaxConcurrency int           // 0 = unlimited
	Timeout        time.Duration
	FailFast       bool // cancel remaining on first error
	MergeFunc      func([]*StageOutput) *StageOutput
}

// FanOut executes stages in parallel and collects results.
func FanOut(ctx context.Context, stages []StageFunc, input StageInput, config ...FanOutConfig) (*OrchestratorResult, error) {
	cfg := FanOutConfig{}
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	// Separate cancel for fail-fast.
	fanCtx, fanCancel := context.WithCancel(ctx)
	defer fanCancel()

	if len(stages) == 0 {
		return &OrchestratorResult{
			Pattern:    "fan-out",
			Outputs:    []*StageOutput{},
			Duration:   0,
			StageCount: 0,
		}, nil
	}

	start := time.Now()
	outputs := make([]*StageOutput, len(stages))
	var mu sync.Mutex
	var firstErr error

	sem := make(chan struct{}, maxConcurrency(cfg.MaxConcurrency, len(stages)))

	var wg sync.WaitGroup
	for i, stage := range stages {
		wg.Add(1)
		go func(idx int, fn StageFunc) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-fanCtx.Done():
				mu.Lock()
				outputs[idx] = &StageOutput{Status: "error", Error: fanCtx.Err().Error()}
				mu.Unlock()
				return
			}

			stageStart := time.Now()
			out, err := fn(fanCtx, input)
			elapsed := time.Since(stageStart)

			if err != nil {
				mu.Lock()
				outputs[idx] = &StageOutput{
					Status:   "error",
					Error:    err.Error(),
					Duration: elapsed,
				}
				if cfg.FailFast && firstErr == nil {
					firstErr = err
					fanCancel()
				}
				mu.Unlock()
				return
			}

			if out != nil {
				out.Duration = elapsed
			}
			mu.Lock()
			outputs[idx] = out
			mu.Unlock()
		}(i, stage)
	}

	wg.Wait()

	result := &OrchestratorResult{
		Pattern:    "fan-out",
		Outputs:    outputs,
		Duration:   time.Since(start),
		StageCount: len(stages),
	}

	if cfg.MergeFunc != nil {
		merged := cfg.MergeFunc(outputs)
		result.Outputs = []*StageOutput{merged}
	}

	return result, firstErr
}

func maxConcurrency(configured, total int) int {
	if configured <= 0 {
		return total
	}
	if configured > total {
		return total
	}
	return configured
}
