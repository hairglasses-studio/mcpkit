package workflow

import (
	"context"
	"maps"
	"testing"

	"github.com/hairglasses-studio/mcpkit/orchestrator"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// --- FromStageFunc ---

func TestFromStageFunc(t *testing.T) {
	stage := func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
		result := make(map[string]any)
		maps.Copy(result, input.Data)
		result["stage_ran"] = true
		return &orchestrator.StageOutput{
			Data:   result,
			Status: "ok",
		}, nil
	}

	fn := FromStageFunc(stage)

	initial := NewState()
	initial.Data["input"] = "hello"

	out, err := fn(context.Background(), initial)
	if err != nil {
		t.Fatalf("FromStageFunc node: %v", err)
	}
	if v, ok := Get[bool](out, "stage_ran"); !ok || !v {
		t.Error("expected stage_ran = true")
	}
	if v, ok := Get[string](out, "input"); !ok || v != "hello" {
		t.Errorf("expected input = hello, got %v", v)
	}
}

func TestFromStageFuncMergesMetadata(t *testing.T) {
	stage := func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
		return &orchestrator.StageOutput{
			Data:     map[string]any{"new_key": "new_val"},
			Metadata: map[string]string{"stage_meta": "yes"},
			Status:   "ok",
		}, nil
	}

	fn := FromStageFunc(stage)
	initial := NewState()
	initial.Metadata["existing"] = "old"

	out, err := fn(context.Background(), initial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Metadata["stage_meta"] != "yes" {
		t.Errorf("expected stage_meta=yes, got %q", out.Metadata["stage_meta"])
	}
	if out.Metadata["existing"] != "old" {
		t.Errorf("expected existing metadata to be preserved, got %q", out.Metadata["existing"])
	}
}

func TestFromStageFuncError(t *testing.T) {
	stage := func(_ context.Context, _ orchestrator.StageInput) (*orchestrator.StageOutput, error) {
		return nil, context.DeadlineExceeded
	}

	fn := FromStageFunc(stage)
	_, err := fn(context.Background(), NewState())
	if err == nil {
		t.Error("expected error from stage")
	}
}

// --- FromToolHandler ---

func TestFromToolHandler(t *testing.T) {
	handler := func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		args := registry.ExtractArguments(req)
		name, _ := args["name"].(string)
		return &registry.CallToolResult{
			Content: []registry.Content{
				registry.MakeTextContent("hello, " + name),
			},
		}, nil
	}

	fn := FromToolHandler("greet", handler)

	state := NewState()
	state.Data["name"] = "world"

	out, err := fn(context.Background(), state)
	if err != nil {
		t.Fatalf("FromToolHandler node: %v", err)
	}

	result, ok := Get[string](out, "tool_result")
	if !ok {
		t.Fatal("expected tool_result in state")
	}
	if result != "hello, world" {
		t.Errorf("tool_result = %q; want %q", result, "hello, world")
	}
}

func TestFromToolHandlerNilResult(t *testing.T) {
	handler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, nil
	}

	fn := FromToolHandler("noop", handler)
	out, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// no tool_result in state
	if _, ok := out.Data["tool_result"]; ok {
		t.Error("expected no tool_result for nil result")
	}
}

func TestFromToolHandlerSetsName(t *testing.T) {
	var gotName string
	handler := func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		gotName = req.Params.Name
		return registry.MakeTextResult("ok"), nil
	}

	fn := FromToolHandler("my-tool", handler)
	_, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "my-tool" {
		t.Errorf("Params.Name = %q; want %q", gotName, "my-tool")
	}
}

// --- SamplingNode ---

type mockSampler struct {
	response *sampling.CreateMessageResult
}

func (m *mockSampler) CreateMessage(_ context.Context, _ sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	return m.response, nil
}

func TestSamplingNode(t *testing.T) {
	response := &sampling.CreateMessageResult{
		SamplingMessage: sampling.SamplingMessage{
			Role:    registry.RoleAssistant,
			Content: registry.TextContent{Type: "text", Text: "LLM says hello"},
		},
		Model: "test-model",
	}

	mock := &mockSampler{response: response}

	fn := SamplingNode(mock, func(s State) sampling.CreateMessageRequest {
		return sampling.CompletionRequest([]sampling.SamplingMessage{
			sampling.TextMessage("user", "say hello"),
		})
	}, "my_output")

	out, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("SamplingNode: %v", err)
	}

	text, ok := Get[string](out, "my_output")
	if !ok {
		t.Fatal("expected my_output in state")
	}
	if text != "LLM says hello" {
		t.Errorf("my_output = %q; want %q", text, "LLM says hello")
	}
}

func TestSamplingNodeDefaultOutputKey(t *testing.T) {
	response := &sampling.CreateMessageResult{
		SamplingMessage: sampling.SamplingMessage{
			Role:    registry.RoleAssistant,
			Content: registry.TextContent{Type: "text", Text: "response"},
		},
	}

	fn := SamplingNode(&mockSampler{response: response}, func(s State) sampling.CreateMessageRequest {
		return sampling.CreateMessageRequest{}
	}, "") // empty key — should use default "llm_response"

	out, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("SamplingNode: %v", err)
	}
	if _, ok := Get[string](out, "llm_response"); !ok {
		t.Error("expected llm_response as default key")
	}
}

func TestSamplingNodeNilResult(t *testing.T) {
	fn := SamplingNode(&mockSampler{response: nil}, func(s State) sampling.CreateMessageRequest {
		return sampling.CreateMessageRequest{}
	}, "out")

	out, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out.Data["out"]; ok {
		t.Error("expected no out key for nil result")
	}
}

func TestSamplingNodeNonTextContent(t *testing.T) {
	// Content is not TextContent — should not set output key
	response := &sampling.CreateMessageResult{
		SamplingMessage: sampling.SamplingMessage{
			Role:    registry.RoleAssistant,
			Content: "not a TextContent",
		},
	}

	fn := SamplingNode(&mockSampler{response: response}, func(s State) sampling.CreateMessageRequest {
		return sampling.CreateMessageRequest{}
	}, "out")

	out, err := fn(context.Background(), NewState())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out.Data["out"]; ok {
		t.Error("expected no out key for non-text content")
	}
}

// --- Integration: FromStageFunc inside a graph ---

func TestFromStageFuncInGraph(t *testing.T) {
	stage := func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
		count, _ := input.Data["count"].(int)
		return &orchestrator.StageOutput{
			Data:   map[string]any{"count": count + 10},
			Status: "ok",
		}, nil
	}

	g := NewGraph()
	if err := g.AddNode("stage", FromStageFunc(stage)); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := g.AddEdge("stage", EndNode); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := g.SetStart("stage"); err != nil {
		t.Fatalf("SetStart: %v", err)
	}

	e, err := NewEngine(g)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	initial := Set(NewState(), "count", 5)
	result, err := e.Run(context.Background(), "run-stage", initial)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Errorf("Status = %v; want completed", result.Status)
	}
	count, _ := Get[int](result.FinalState, "count")
	if count != 15 {
		t.Errorf("count = %d; want 15", count)
	}
}
