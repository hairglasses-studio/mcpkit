//go:build !official_sdk

package ralph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// Run executes the autonomous loop until completion, stop, or max iterations.
func (l *Loop) Run(ctx context.Context) error {
	var spec Spec
	var err error
	if len(l.config.TemplateVars) > 0 {
		spec, err = RenderSpec(l.config.SpecFile, l.config.TemplateVars)
	} else {
		spec, err = LoadSpec(l.config.SpecFile)
	}
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.progress, err = LoadProgress(l.config.ProgressFile)
	if err != nil {
		l.mu.Unlock()
		return err
	}
	if l.config.ForceRestart {
		l.progress = Progress{}
	}
	if l.progress.Status == StatusCompleted {
		l.mu.Unlock()
		return nil
	}
	l.progress.SpecFile = l.config.SpecFile
	l.progress.Status = StatusRunning
	if l.progress.StartedAt.IsZero() {
		l.progress.StartedAt = time.Now()
	}
	l.mu.Unlock()

	for {
		// Check stop conditions.
		select {
		case <-ctx.Done():
			l.setStatus(StatusStopped)
			return ctx.Err()
		case <-l.stopCh:
			l.setStatus(StatusStopped)
			return nil
		default:
		}

		l.mu.Lock()
		iteration := l.progress.Iteration + 1
		if iteration > l.config.MaxIterations {
			l.progress.Status = StatusFailed
			l.progress.UpdatedAt = time.Now()
			SaveProgress(l.config.ProgressFile, l.progress)
			l.mu.Unlock()
			return fmt.Errorf("ralph: max iterations (%d) reached", l.config.MaxIterations)
		}
		l.progress.Iteration = iteration
		progressCopy := l.progress
		l.mu.Unlock()

		l.config.Hooks.callIterationStart(iteration)

		// Re-read spec each iteration (clean context principle).
		if len(l.config.TemplateVars) > 0 {
			spec, err = RenderSpec(l.config.SpecFile, l.config.TemplateVars)
		} else {
			spec, err = LoadSpec(l.config.SpecFile)
		}
		if err != nil {
			l.setStatus(StatusFailed)
			return err
		}

		// Build prompt and call LLM.
		tools := l.config.ToolRegistry.GetAllToolDefinitions()
		prompt := buildIterationPrompt(spec, progressCopy, tools)

		messages := []sampling.SamplingMessage{
			sampling.TextMessage("user", prompt),
		}
		opts := []sampling.RequestOption{
			sampling.WithMaxTokens(l.config.MaxTokens),
			sampling.WithSystemPrompt(systemPrompt),
		}
		if l.config.ModelSelector != nil {
			if model := l.config.ModelSelector(iteration, progressCopy.CompletedIDs); model != "" {
				opts = append(opts, sampling.WithModel(model))
			}
		}
		req := sampling.CompletionRequest(messages, opts...)

		result, err := l.config.Sampler.CreateMessage(ctx, req)
		if err != nil {
			l.recordIteration(iteration, "", nil, fmt.Sprintf("sampler error: %v", err))
			continue
		}

		// Extract text from result.
		// result.Content is SamplingMessage.Content which is typed as `any`.
		// We must type-assert to registry.Content before passing to ExtractTextContent.
		content, ok := result.Content.(registry.Content)
		if !ok {
			l.recordIteration(iteration, "", nil, "no text in sampler response")
			continue
		}
		responseText, ok := registry.ExtractTextContent(content)
		if !ok {
			l.recordIteration(iteration, "", nil, "no text in sampler response")
			continue
		}

		// Parse decision.
		decision, err := parseDecision(responseText)
		if err != nil {
			l.recordIteration(iteration, "", nil, fmt.Sprintf("parse error: %v", err))
			continue
		}

		// Complete?
		if decision.Complete {
			l.recordIteration(iteration, "", nil, "loop completed")
			l.setStatus(StatusCompleted)
			return nil
		}

		// Resolve tool calls (multi-tool or single-tool shim).
		calls := decision.ResolvedToolCalls()
		if len(calls) == 0 {
			l.recordIteration(iteration, decision.TaskID, nil, "no tool specified in decision")
			l.recordCost(iteration, prompt, responseText, nil, nil)
			continue
		}

		// Execute each tool call sequentially.
		var toolNames []string
		var resultParts []string
		var toolResults []*registry.CallToolResult

		for _, call := range calls {
			td, found := l.config.ToolRegistry.GetTool(call.Name)
			if !found {
				l.config.Hooks.callError(iteration, fmt.Errorf("tool %q not found", call.Name))
				toolNames = append(toolNames, call.Name)
				resultParts = append(resultParts, fmt.Sprintf("tool %q not found", call.Name))
				toolResults = append(toolResults, nil)
				continue
			}

			toolReq := makeCallToolRequest(call.Name, call.Arguments)
			toolResult, err := td.Handler(ctx, toolReq)

			toolNames = append(toolNames, call.Name)
			toolResults = append(toolResults, toolResult)

			if err != nil {
				l.config.Hooks.callError(iteration, err)
				resultParts = append(resultParts, fmt.Sprintf("tool error: %v", err))
			} else if toolResult != nil && len(toolResult.Content) > 0 {
				if text, ok := registry.ExtractTextContent(toolResult.Content[0]); ok {
					if toolResult.IsError {
						resultParts = append(resultParts, "tool error: "+text)
					} else {
						resultParts = append(resultParts, text)
					}
				} else {
					resultParts = append(resultParts, "tool returned non-text content")
				}
			} else {
				resultParts = append(resultParts, "tool returned empty result")
			}
		}

		combinedResult := strings.Join(resultParts, "\n")

		// Mark task done if requested.
		if decision.MarkDone && decision.TaskID != "" {
			l.mu.Lock()
			l.progress.CompletedIDs = appendUnique(l.progress.CompletedIDs, decision.TaskID)
			l.mu.Unlock()
			l.config.Hooks.callTaskComplete(decision.TaskID)
		}

		l.recordIteration(iteration, decision.TaskID, toolNames, combinedResult)
		l.recordCost(iteration, prompt, responseText, calls, toolResults)
	}
}

func makeCallToolRequest(name string, args map[string]interface{}) registry.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

func (l *Loop) setStatus(s Status) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.progress.Status = s
	l.progress.UpdatedAt = time.Now()
	SaveProgress(l.config.ProgressFile, l.progress)
}

func (l *Loop) recordIteration(iteration int, taskID string, toolCalls []string, result string) {
	l.mu.Lock()
	entry := IterationLog{
		Iteration: iteration,
		TaskID:    taskID,
		ToolCalls: toolCalls,
		Result:    result,
		Timestamp: time.Now(),
	}
	l.progress.Log = append(l.progress.Log, entry)
	l.progress.UpdatedAt = time.Now()
	SaveProgress(l.config.ProgressFile, l.progress)
	l.mu.Unlock()
	l.config.Hooks.callIterationEnd(entry)
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// recordCost records token usage for a completed iteration when a CostTracker is configured.
func (l *Loop) recordCost(iteration int, prompt, response string, calls []ToolCall, results []*registry.CallToolResult) {
	if l.config.CostTracker == nil {
		return
	}
	estimate := l.config.EstimateFunc
	if estimate == nil {
		estimate = func(text string) int { return len(text) / 4 }
	}

	// Record sampling usage (prompt + response together as ralph/sampling entry).
	l.config.CostTracker.Record(finops.UsageEntry{
		ToolName:     "ralph/sampling",
		Category:     "sampling",
		InputTokens:  estimate(prompt),
		OutputTokens: estimate(response),
		Timestamp:    time.Now(),
	})

	// Record per-tool usage.
	for i, call := range calls {
		inputTokens := 0
		outputTokens := 0
		if call.Arguments != nil {
			if argBytes, err := json.Marshal(call.Arguments); err == nil {
				inputTokens = estimate(string(argBytes))
			}
		}
		if i < len(results) && results[i] != nil {
			outputTokens = finops.EstimateFromResult(results[i], estimate)
		}
		l.config.CostTracker.Record(finops.UsageEntry{
			ToolName:     call.Name,
			Category:     "tool",
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Timestamp:    time.Now(),
		})
	}

	l.config.Hooks.callCostUpdate(iteration, l.config.CostTracker.Summary())
}
