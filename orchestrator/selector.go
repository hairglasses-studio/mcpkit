package orchestrator

import (
	"context"
	"errors"
	"sync"
	"time"
)

// SelectorConfig configures best-of-N selection.
type SelectorConfig struct {
	MaxConcurrency int
	Timeout        time.Duration
	Scorer         func(*StageOutput) float64 // required
}

// Select runs stages in parallel, picks the best by score.
func Select(ctx context.Context, stages []StageFunc, input StageInput, config SelectorConfig) (*OrchestratorResult, error) {
	if config.Scorer == nil {
		return nil, errors.New("orchestrator: scorer is required for Select")
	}

	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	start := time.Now()
	outputs := make([]*StageOutput, len(stages))
	sem := make(chan struct{}, maxConcurrency(config.MaxConcurrency, len(stages)))

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, stage := range stages {
		wg.Add(1)
		go func(idx int, fn StageFunc) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				outputs[idx] = &StageOutput{Status: "error", Error: ctx.Err().Error()}
				mu.Unlock()
				return
			}

			stageStart := time.Now()
			out, err := fn(ctx, input)
			elapsed := time.Since(stageStart)

			if err != nil {
				mu.Lock()
				outputs[idx] = &StageOutput{Status: "error", Error: err.Error(), Duration: elapsed}
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

	// Score and pick best.
	bestIdx := -1
	bestScore := -1.0
	for i, out := range outputs {
		if out == nil || out.Status == "error" {
			continue
		}
		score := config.Scorer(out)
		if bestIdx == -1 || score > bestScore {
			bestIdx = i
			bestScore = score
		}
	}

	result := &OrchestratorResult{
		Pattern:    "select",
		Outputs:    outputs,
		Duration:   time.Since(start),
		StageCount: len(stages),
	}

	// Mark the best output in metadata.
	if bestIdx >= 0 {
		selected := outputs[bestIdx]
		if selected.Metadata == nil {
			selected.Metadata = make(map[string]string)
		}
		selected.Metadata["selected"] = "true"
	}

	return result, nil
}
