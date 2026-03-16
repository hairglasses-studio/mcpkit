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
	l.progress.ProjectRoot = l.config.ProjectRoot
	l.progress.Status = StatusRunning
	if l.progress.StartedAt.IsZero() {
		l.progress.StartedAt = time.Now()
	}
	l.mu.Unlock()

	// Load checkpoint for conversation history resumption.
	if l.config.HistoryWindow > 0 {
		cpFile := l.config.CheckpointFile
		if cpFile == "" {
			cpFile = DefaultCheckpointFile(l.config.SpecFile)
		}
		if !l.config.ForceRestart {
			if loaded, cpErr := LoadCheckpoint(cpFile); cpErr == nil && len(loaded) > 0 {
				l.history = loaded
			}
		}
		// Store resolved checkpoint file for saving later.
		l.checkpointFile = cpFile
	}

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

		// Circuit breaker: block if the circuit is open.
		if l.config.CircuitBreaker != nil && !l.config.CircuitBreaker.CanExecute() {
			l.setStatus(StatusStopped)
			return fmt.Errorf("ralph: circuit breaker open: %s", l.config.CircuitBreaker.OpenReason())
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

		// Re-read spec each iteration (clean context principle),
		// unless the TaskDecomposer has modified it in-memory.
		if !l.specModified {
			if len(l.config.TemplateVars) > 0 {
				spec, err = RenderSpec(l.config.SpecFile, l.config.TemplateVars)
			} else {
				spec, err = LoadSpec(l.config.SpecFile)
			}
			if err != nil {
				l.setStatus(StatusFailed)
				return err
			}
		}

		// Build prompt and call LLM.
		tools := l.config.ToolRegistry.GetAllToolDefinitions()
		l.mu.Lock()
		currentStuckHint := l.stuckHint
		l.mu.Unlock()
		prompt := buildIterationPrompt(spec, progressCopy, tools, currentStuckHint)

		// Determine max tokens: phase override > config default.
		maxTokens := l.config.MaxTokens
		if l.config.PhaseMaxTokens != nil {
			// Find the first ready task to use as phase key.
			completed := make(map[string]bool)
			for _, id := range progressCopy.CompletedIDs {
				completed[id] = true
			}
			for _, id := range ReadyTasks(spec.Tasks, completed) {
				if override, ok := l.config.PhaseMaxTokens[id]; ok {
					maxTokens = override
					break
				}
			}
		}

		// Build messages: multi-turn history or single-turn.
		var messages []sampling.SamplingMessage
		if l.config.HistoryWindow > 0 && len(l.history) > 0 {
			messages = BuildMessages(l.history, l.config.HistoryWindow, prompt)
		} else {
			messages = []sampling.SamplingMessage{
				sampling.TextMessage("user", prompt),
			}
		}

		opts := []sampling.RequestOption{
			sampling.WithMaxTokens(maxTokens),
			sampling.WithSystemPrompt(systemPrompt),
		}
		if l.config.ModelSelector != nil {
			if model := l.config.ModelSelector(iteration, progressCopy.CompletedIDs); model != "" {
				opts = append(opts, sampling.WithModel(model))
			}
		}
		req := sampling.CompletionRequest(messages, opts...)

		// Sampler call with retry/backoff.
		var result *sampling.CreateMessageResult
		var samplerErr error
		for attempt := 0; attempt <= l.config.SamplerRetries; attempt++ {
			if attempt > 0 {
				backoff := l.config.SamplerBackoff * time.Duration(1<<(attempt-1))
				select {
				case <-ctx.Done():
					l.setStatus(StatusStopped)
					return ctx.Err()
				case <-l.stopCh:
					l.setStatus(StatusStopped)
					return nil
				case <-time.After(backoff):
				}
			}
			result, samplerErr = l.config.Sampler.CreateMessage(ctx, req)
			if samplerErr == nil {
				break
			}
		}
		if samplerErr != nil {
			l.mu.Lock()
			l.consecutiveSamplerFails++
			fails := l.consecutiveSamplerFails
			l.mu.Unlock()
			l.recordIteration(iteration, "", nil,
				fmt.Sprintf("sampler error (after %d retries, %d consecutive): %v", l.config.SamplerRetries, fails, samplerErr))
			if l.config.MaxConsecutiveSamplerFailures > 0 && fails >= l.config.MaxConsecutiveSamplerFailures {
				l.setStatus(StatusFailed)
				return fmt.Errorf("ralph: %d consecutive sampler failures, last error: %v", fails, samplerErr)
			}
			continue
		}
		// Reset consecutive failure counter on success.
		l.mu.Lock()
		l.consecutiveSamplerFails = 0
		l.mu.Unlock()

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
			preview := responseText
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			l.recordIteration(iteration, "", nil,
				fmt.Sprintf("parse error: %v\nRaw response (truncated):\n%s", err, preview))
			continue
		}

		// Reject decisions targeting blocked tasks (dependencies not met).
		if decision.TaskID != "" {
			completed := make(map[string]bool)
			for _, id := range progressCopy.CompletedIDs {
				completed[id] = true
			}
			if !completed[decision.TaskID] {
				readyIDs := ReadyTasks(spec.Tasks, completed)
				readySet := make(map[string]bool)
				for _, id := range readyIDs {
					readySet[id] = true
				}
				if !readySet[decision.TaskID] {
					readyList := strings.Join(readyIDs, ", ")
					l.recordIteration(iteration, decision.TaskID, nil,
						fmt.Sprintf("task %q is blocked (dependencies not met). Ready tasks: [%s]", decision.TaskID, readyList))
					continue
				}
			}
		}

		// Complete?
		if decision.Complete {
			// ExitGate: require all task IDs done before accepting completion.
			if l.config.ExitGate.RequireAllTasksDone {
				l.mu.Lock()
				doneSet := make(map[string]bool)
				for _, id := range l.progress.CompletedIDs {
					doneSet[id] = true
				}
				allDone := true
				var missing []string
				for _, t := range spec.Tasks {
					if !doneSet[t.ID] {
						allDone = false
						missing = append(missing, t.ID)
					}
				}
				l.mu.Unlock()
				if !allDone {
					l.recordIteration(iteration, "", nil,
						fmt.Sprintf("completion rejected: ExitGate requires all tasks done; missing: %s",
							strings.Join(missing, ", ")))
					continue
				}
			}
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
				availableNames := l.config.ToolRegistry.ListTools()
				resultParts = append(resultParts,
					fmt.Sprintf("tool %q not found. Available tools: [%s]", call.Name, strings.Join(availableNames, ", ")))
				toolResults = append(toolResults, nil)
				continue
			}

			toolReq := makeCallToolRequest(call.Name, call.Arguments)
			toolCtx, toolCancel := context.WithTimeout(ctx, l.config.ToolTimeout)
			toolResult, err := td.Handler(toolCtx, toolReq)
			toolCancel()

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

		// Auto-verify: run checks after write_file calls based on configured level.
		if l.config.AutoVerifyLevel != "" && l.config.ProjectRoot != "" {
			seenPkgs := make(map[string]bool)
			for _, call := range calls {
				if call.Name != "write_file" {
					continue
				}
				path, _ := call.Arguments["path"].(string)
				pkg := detectPackage(path)
				if pkg == "" || seenPkgs[pkg] {
					continue
				}
				seenPkgs[pkg] = true
				verifyResults := runAutoVerify(ctx, l.config.ProjectRoot, pkg, l.config.AutoVerifyLevel)
				resultParts = append(resultParts, verifyResults...)
			}
			combinedResult = strings.Join(resultParts, "\n")
		}

		// Record conversation turn for multi-turn history.
		if l.config.HistoryWindow > 0 {
			l.history = append(l.history, ConversationTurn{
				UserPrompt:    prompt,
				AssistantText: responseText,
				ToolResults:   resultParts,
			})
			// Prune to bound file size: keep 2x window.
			l.history = pruneHistory(l.history, l.config.HistoryWindow*2)
			// Persist checkpoint.
			if l.checkpointFile != "" {
				SaveCheckpoint(l.checkpointFile, l.history)
			}
		}

		// Mark task done if requested, but only when the task is ready (not blocked).
		if decision.MarkDone && decision.TaskID != "" {
			l.mu.Lock()
			doneSet := make(map[string]bool)
			for _, id := range l.progress.CompletedIDs {
				doneSet[id] = true
			}
			allowMark := doneSet[decision.TaskID]
			if !allowMark {
				readyNow := ReadyTasks(spec.Tasks, doneSet)
				for _, id := range readyNow {
					if id == decision.TaskID {
						allowMark = true
						break
					}
				}
			}
			if allowMark {
				l.progress.CompletedIDs = appendUnique(l.progress.CompletedIDs, decision.TaskID)
				l.mu.Unlock()
				l.config.Hooks.callTaskComplete(decision.TaskID)

				// Task decomposition: inject sub-tasks after completion.
				if l.config.TaskDecomposer != nil {
					l.mu.Lock()
					subTasks := l.config.TaskDecomposer(decision.TaskID, l.progress, &spec)
					l.mu.Unlock()
					if len(subTasks) > 0 {
						spec.Tasks = injectSubTasks(spec.Tasks, decision.TaskID, subTasks)
						l.specModified = true
					}
				}
			} else {
				l.mu.Unlock()
			}
		}

		l.recordIteration(iteration, decision.TaskID, toolNames, combinedResult)
		l.recordCost(iteration, prompt, responseText, calls, toolResults)

		// Stuck-loop detection: inject corrective hint if patterns detected.
		stuckThreshold := l.config.StuckThreshold
		if stuckThreshold <= 0 {
			stuckThreshold = 3
		}
		detector := NewStuckDetector(stuckThreshold)
		l.mu.Lock()
		if sig := detector.Check(l.progress.Log); sig != nil {
			l.stuckHint = sig.Suggestion
		} else {
			l.stuckHint = ""
		}
		l.mu.Unlock()

		// Circuit breaker: record this iteration's result.
		if l.config.CircuitBreaker != nil {
			hasProgress := decision.MarkDone && decision.TaskID != ""
			errorKey := ""
			if isErrorResult(combinedResult) {
				errorKey = normalizeError(combinedResult)
			}
			l.config.CircuitBreaker.RecordResult(hasProgress, errorKey)
		}

		// Budget guard: use CostGovernor when configured, else fall back to legacy BudgetLimit.
		if l.config.CostGovernor != nil {
			// Estimate tokens for this iteration: prompt + response.
			estimate := l.config.EstimateFunc
			if estimate == nil {
				estimate = func(text string) int { return len(text) / 4 }
			}
			tokens := int64(estimate(prompt) + estimate(responseText))
			hasProgress := decision.MarkDone && decision.TaskID != ""
			l.config.CostGovernor.RecordIteration(tokens, hasProgress)
			verdict := l.config.CostGovernor.Check()
			switch verdict.Action {
			case "halt":
				l.setStatus(StatusFailed)
				return fmt.Errorf("ralph: cost governor halt: %s", verdict.Warning)
			case "downgrade":
				l.mu.Lock()
				l.costDowngrade = true
				l.mu.Unlock()
			}
		} else if l.config.BudgetLimit > 0 && l.config.CostTracker != nil {
			if l.config.CostTracker.Total() > l.config.BudgetLimit {
				l.setStatus(StatusFailed)
				return fmt.Errorf("ralph: token budget exceeded (limit=%d, used=%d)",
					l.config.BudgetLimit, l.config.CostTracker.Total())
			}
		}
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

// injectSubTasks replaces the completed task entry with sub-tasks that depend on
// the same dependencies as the original, preserving DAG ordering.
func injectSubTasks(tasks []Task, completedID string, subTasks []Task) []Task {
	var result []Task
	for _, t := range tasks {
		if t.ID == completedID {
			// Replace with sub-tasks at the same position.
			result = append(result, subTasks...)
		} else {
			result = append(result, t)
		}
	}
	return result
}
