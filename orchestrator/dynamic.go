package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DynamicPattern is a supported orchestration pattern selected at runtime.
type DynamicPattern string

const (
	DynamicPatternFanOut   DynamicPattern = "fan-out"
	DynamicPatternPipeline DynamicPattern = "pipeline"
	DynamicPatternSelect   DynamicPattern = "select"
)

// DynamicPatternDecision explains the selected pattern.
type DynamicPatternDecision struct {
	Pattern DynamicPattern `json:"pattern"`
	Reason  string         `json:"reason"`
}

// DynamicPatternConfig configures RunDynamicPattern.
type DynamicPatternConfig struct {
	FanOut   FanOutConfig
	Pipeline PipelineConfig
	Selector SelectorConfig
}

var ErrUnknownDynamicPattern = errors.New("orchestrator: unknown dynamic pattern")

// SelectDynamicPattern chooses fan-out, pipeline, or select from runtime metadata.
//
// Supported metadata:
//   - orchestrator.pattern: explicit fan-out, pipeline, or select override
//   - orchestrator.best_of: true selects the select pattern
//   - orchestrator.ordered or orchestrator.depends_on_previous: true selects pipeline
//   - orchestrator.parallel: false selects pipeline; true selects fan-out
func SelectDynamicPattern(input StageInput, stageCount int) (DynamicPatternDecision, error) {
	meta := input.Metadata
	if pattern := strings.TrimSpace(meta["orchestrator.pattern"]); pattern != "" {
		switch DynamicPattern(pattern) {
		case DynamicPatternFanOut, DynamicPatternPipeline, DynamicPatternSelect:
			return DynamicPatternDecision{Pattern: DynamicPattern(pattern), Reason: "metadata override"}, nil
		default:
			return DynamicPatternDecision{}, fmt.Errorf("%w: %s", ErrUnknownDynamicPattern, pattern)
		}
	}
	if truthy(meta["orchestrator.best_of"]) {
		return DynamicPatternDecision{Pattern: DynamicPatternSelect, Reason: "best-of metadata"}, nil
	}
	if truthy(meta["orchestrator.ordered"]) || truthy(meta["orchestrator.depends_on_previous"]) {
		return DynamicPatternDecision{Pattern: DynamicPatternPipeline, Reason: "ordered metadata"}, nil
	}
	if falsy(meta["orchestrator.parallel"]) {
		return DynamicPatternDecision{Pattern: DynamicPatternPipeline, Reason: "parallel disabled"}, nil
	}
	if truthy(meta["orchestrator.parallel"]) || stageCount > 1 {
		return DynamicPatternDecision{Pattern: DynamicPatternFanOut, Reason: "parallel default"}, nil
	}
	return DynamicPatternDecision{Pattern: DynamicPatternPipeline, Reason: "single-stage default"}, nil
}

// RunDynamicPattern selects and executes an orchestration pattern from input metadata.
func RunDynamicPattern(ctx context.Context, stages []StageFunc, input StageInput, config DynamicPatternConfig) (*OrchestratorResult, DynamicPatternDecision, error) {
	decision, err := SelectDynamicPattern(input, len(stages))
	if err != nil {
		return nil, decision, err
	}
	var result *OrchestratorResult
	switch decision.Pattern {
	case DynamicPatternFanOut:
		result, err = FanOut(ctx, stages, input, config.FanOut)
	case DynamicPatternPipeline:
		result, err = Pipeline(ctx, stages, input, config.Pipeline)
	case DynamicPatternSelect:
		result, err = Select(ctx, stages, input, config.Selector)
	default:
		err = fmt.Errorf("%w: %s", ErrUnknownDynamicPattern, decision.Pattern)
	}
	return result, decision, err
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func falsy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "f", "false", "n", "no", "off":
		return true
	default:
		return false
	}
}
