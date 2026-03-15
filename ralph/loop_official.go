//go:build official_sdk

package ralph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// Run executes the autonomous loop until completion, stop, or max iterations.
func (l *Loop) Run(ctx context.Context) error {
	spec, err := LoadSpec(l.config.SpecFile)
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.progress, err = LoadProgress(l.config.ProgressFile)
	if err != nil {
		l.mu.Unlock()
		return err
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

		// Re-read spec each iteration (clean context principle).
		spec, err = LoadSpec(l.config.SpecFile)
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
		req := sampling.CompletionRequest(messages,
			sampling.WithMaxTokens(l.config.MaxTokens),
			sampling.WithSystemPrompt(systemPrompt),
		)

		result, err := l.config.Sampler.CreateMessage(ctx, req)
		if err != nil {
			l.recordIteration(iteration, "", nil, fmt.Sprintf("sampler error: %v", err))
			continue
		}

		// Extract text from result.
		responseText, ok := registry.ExtractTextContent(result.Content)
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

		// Execute tool.
		if decision.ToolName == "" {
			l.recordIteration(iteration, decision.TaskID, nil, "no tool specified in decision")
			continue
		}

		td, found := l.config.ToolRegistry.GetTool(decision.ToolName)
		if !found {
			l.recordIteration(iteration, decision.TaskID, []string{decision.ToolName}, fmt.Sprintf("tool %q not found", decision.ToolName))
			continue
		}

		toolReq := makeCallToolRequest(decision.ToolName, decision.Arguments)
		toolResult, err := td.Handler(ctx, toolReq)

		var resultText string
		if err != nil {
			resultText = fmt.Sprintf("tool error: %v", err)
		} else if toolResult != nil && len(toolResult.Content) > 0 {
			if text, ok := registry.ExtractTextContent(toolResult.Content[0]); ok {
				resultText = text
			} else {
				resultText = "tool returned non-text content"
			}
			if toolResult.IsError {
				resultText = "tool error: " + resultText
			}
		} else {
			resultText = "tool returned empty result"
		}

		// Mark task done if requested.
		if decision.MarkDone && decision.TaskID != "" {
			l.mu.Lock()
			l.progress.CompletedIDs = appendUnique(l.progress.CompletedIDs, decision.TaskID)
			l.mu.Unlock()
		}

		l.recordIteration(iteration, decision.TaskID, []string{decision.ToolName}, resultText)
	}
}

func makeCallToolRequest(name string, args map[string]interface{}) registry.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params = &mcp.CallToolParamsRaw{Name: name}
	if args != nil {
		argBytes, _ := json.Marshal(args)
		req.Params.Arguments = argBytes
	}
	return req
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
	defer l.mu.Unlock()
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
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
