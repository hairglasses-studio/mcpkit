//go:build !official_sdk

package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/hitools"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// templateNodeFunc creates a NodeFunc that records which stage executed and stores
// the filtered tool definitions under stageKey in State.Data. This is the building
// block for all template pipelines: each stage gets a categorised subset of tools
// and leaves a trace of its execution for downstream stages.
func templateNodeFunc(stageName, stageKey string, tools []registry.ToolDefinition, filter func(registry.ToolDefinition) bool) NodeFunc {
	var filtered []registry.ToolDefinition
	for _, td := range tools {
		if filter(td) {
			filtered = append(filtered, td)
		}
	}
	return func(_ context.Context, state State) (State, error) {
		out := state.Clone()
		out.Data[stageKey+"_tools"] = filtered
		out.Data[stageKey+"_status"] = "completed"

		// Accumulate stage execution order for observability.
		var stages []string
		if existing, ok := Get[[]string](state, "stages_completed"); ok {
			stages = existing
		}
		stages = append(stages, stageName)
		out.Data["stages_completed"] = stages
		return out, nil
	}
}

// toolMatchesAny returns true if the tool's category, tags, or name contain any
// of the given keywords (case-insensitive).
func toolMatchesAny(td registry.ToolDefinition, keywords []string) bool {
	cat := strings.ToLower(td.Category)
	name := strings.ToLower(td.Tool.Name)
	desc := strings.ToLower(td.Tool.Description)
	for _, kw := range keywords {
		kw = strings.ToLower(kw)
		if strings.Contains(cat, kw) || strings.Contains(name, kw) || strings.Contains(desc, kw) {
			return true
		}
		for _, tag := range td.Tags {
			if strings.Contains(strings.ToLower(tag), kw) {
				return true
			}
		}
	}
	return false
}

// CodeReviewPipeline creates a 3-stage code review workflow:
//
//	Stage 1: Static analysis agent (lint, vet, security scan)
//	Stage 2: Logic review agent (correctness, edge cases)
//	Stage 3: Style review agent (conventions, naming, docs)
//
// Each stage receives a filtered subset of tools relevant to its task and stores
// its results in State.Data under "static_analysis", "logic_review", and
// "style_review" prefixed keys respectively. The pipeline executes linearly:
// static_analysis -> logic_review -> style_review -> END.
func CodeReviewPipeline(tools []registry.ToolDefinition) *Graph {
	g := NewGraph()

	staticKeywords := []string{"lint", "vet", "security", "scan", "static", "analyze", "check"}
	logicKeywords := []string{"test", "review", "logic", "verify", "validate", "correctness"}
	styleKeywords := []string{"style", "format", "convention", "doc", "naming", "lint"}

	_ = g.AddNode("static_analysis", templateNodeFunc(
		"static_analysis", "static_analysis", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, staticKeywords)
		},
	))

	_ = g.AddNode("logic_review", templateNodeFunc(
		"logic_review", "logic_review", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, logicKeywords)
		},
	))

	_ = g.AddNode("style_review", templateNodeFunc(
		"style_review", "style_review", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, styleKeywords)
		},
	))

	_ = g.AddEdge("static_analysis", "logic_review")
	_ = g.AddEdge("logic_review", "style_review")
	_ = g.AddEdge("style_review", EndNode)
	_ = g.SetStart("static_analysis")

	return g
}

// ResearchWritePipeline creates a research-then-write workflow:
//
//	Stage 1: Research agent (search, read, gather sources)
//	Stage 2: Synthesis agent (analyze, identify patterns)
//	Stage 3: Write agent (draft, format, cite sources)
//
// Each stage receives a filtered subset of tools relevant to its task. The
// pipeline executes linearly: research -> synthesis -> write -> END. Stage
// results are accumulated so downstream stages can build on earlier findings.
func ResearchWritePipeline(tools []registry.ToolDefinition) *Graph {
	g := NewGraph()

	researchKeywords := []string{"search", "read", "fetch", "gather", "browse", "query", "source"}
	synthesisKeywords := []string{"analyze", "synthesis", "pattern", "summarize", "compare", "classify"}
	writeKeywords := []string{"write", "draft", "format", "cite", "publish", "render", "output"}

	_ = g.AddNode("research", templateNodeFunc(
		"research", "research", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, researchKeywords)
		},
	))

	_ = g.AddNode("synthesis", templateNodeFunc(
		"synthesis", "synthesis", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, synthesisKeywords)
		},
	))

	_ = g.AddNode("write", templateNodeFunc(
		"write", "write", tools,
		func(td registry.ToolDefinition) bool {
			return toolMatchesAny(td, writeKeywords)
		},
	))

	_ = g.AddEdge("research", "synthesis")
	_ = g.AddEdge("synthesis", "write")
	_ = g.AddEdge("write", EndNode)
	_ = g.SetStart("research")

	return g
}

// ApprovalGatePipeline creates a workflow with human approval:
//
//	Stage 1: Analyze agent (assess risk, classify action)
//	Stage 2: Approval gate (human decision: approve/deny/modify)
//	Stage 3: Execute agent (perform action if approved)
//
// The approval gate submits an ApprovalRequest to the provided store and polls
// for a response. If the human denies the request, the pipeline terminates with
// the denial recorded in state. If approved (or modified), the execute stage runs.
//
// State keys used:
//   - "approval_action": set by the analyze stage (describes what needs approval)
//   - "approval_request_id": the ID of the submitted approval request
//   - "approval_decision": the human's decision (approved/denied/modified)
//   - "approval_comment": optional human comment
//   - "execute_status": set by the execute stage on completion
func ApprovalGatePipeline(tools []registry.ToolDefinition, approvalStore hitools.ApprovalStore) *Graph {
	g := NewGraph()

	analyzeKeywords := []string{"analyze", "assess", "risk", "classify", "evaluate", "inspect", "check"}
	executeKeywords := []string{"execute", "run", "apply", "deploy", "write", "create", "update", "delete"}

	// Stage 1: Analyze — assess what needs to happen and describe the action.
	_ = g.AddNode("analyze", func(ctx context.Context, state State) (State, error) {
		out := state.Clone()

		// Filter tools for analysis stage.
		var analyzeTools []registry.ToolDefinition
		for _, td := range tools {
			if toolMatchesAny(td, analyzeKeywords) {
				analyzeTools = append(analyzeTools, td)
			}
		}
		out.Data["analyze_tools"] = analyzeTools
		out.Data["analyze_status"] = "completed"

		// Build a default action description from state if not already set.
		if _, ok := out.Data["approval_action"]; !ok {
			out.Data["approval_action"] = "execute workflow action"
		}

		var stages []string
		if existing, ok := Get[[]string](state, "stages_completed"); ok {
			stages = existing
		}
		stages = append(stages, "analyze")
		out.Data["stages_completed"] = stages
		return out, nil
	})

	// Stage 2: Approval gate — submit request and wait for human decision.
	_ = g.AddNode("approval_gate", func(ctx context.Context, state State) (State, error) {
		out := state.Clone()

		action, _ := Get[string](state, "approval_action")
		if action == "" {
			action = "execute workflow action"
		}

		req := hitools.ApprovalRequest{
			ID:        fmt.Sprintf("wf-approval-%d", time.Now().UnixNano()),
			ToolName:  "workflow_execute",
			Action:    action,
			Context:   "Workflow approval gate: human decision required before execution stage.",
			Urgency:   hitools.ApprovalUrgencyNormal,
			CreatedAt: time.Now(),
		}

		if err := approvalStore.Submit(ctx, req); err != nil {
			return state, fmt.Errorf("approval gate: submit: %w", err)
		}

		out.Data["approval_request_id"] = req.ID

		// Poll for response with back-off.
		resp, err := pollApproval(ctx, approvalStore, req.ID)
		if err != nil {
			return state, fmt.Errorf("approval gate: poll: %w", err)
		}

		out.Data["approval_decision"] = string(resp.Decision)
		out.Data["approval_comment"] = resp.Comment

		var stages []string
		if existing, ok := Get[[]string](state, "stages_completed"); ok {
			stages = existing
		}
		stages = append(stages, "approval_gate")
		out.Data["stages_completed"] = stages
		return out, nil
	})

	// Stage 3: Execute — runs only when approval is granted.
	_ = g.AddNode("execute", func(ctx context.Context, state State) (State, error) {
		out := state.Clone()

		var executeTools []registry.ToolDefinition
		for _, td := range tools {
			if toolMatchesAny(td, executeKeywords) {
				executeTools = append(executeTools, td)
			}
		}
		out.Data["execute_tools"] = executeTools
		out.Data["execute_status"] = "completed"

		var stages []string
		if existing, ok := Get[[]string](state, "stages_completed"); ok {
			stages = existing
		}
		stages = append(stages, "execute")
		out.Data["stages_completed"] = stages
		return out, nil
	})

	// Edges: analyze -> approval_gate -> (conditional) -> execute or END.
	_ = g.AddEdge("analyze", "approval_gate")
	_ = g.AddConditionalEdge("approval_gate", func(state State) string {
		decision, _ := Get[string](state, "approval_decision")
		switch hitools.Decision(decision) {
		case hitools.Approved, hitools.Modified:
			return "execute"
		default:
			return EndNode
		}
	})
	_ = g.AddEdge("execute", EndNode)
	_ = g.SetStart("analyze")

	return g
}

// pollApproval polls the approval store for a response, using exponential
// back-off from 10ms to 200ms. It respects context cancellation.
func pollApproval(ctx context.Context, store hitools.ApprovalStore, requestID string) (*hitools.ApprovalResponse, error) {
	interval := 10 * time.Millisecond
	maxInterval := 200 * time.Millisecond

	for {
		resp, err := store.GetResponse(ctx, requestID)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("approval request %q: %w", requestID, ctx.Err())
		case <-time.After(interval):
		}

		if interval < maxInterval {
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}
