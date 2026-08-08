package a2a

import (
	"errors"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// newTestTool builds a registry.Tool for fixtures below, routed entirely
// through registry's SDK-neutral Tool/MakeToolInputSchema so this whole file
// needs no SDK import and compiles identically under both build tags (the
// P52.6/P52.7 bridge/a2a port). properties/required may be nil for a
// no-parameter tool.
func newTestTool(name, description string, properties map[string]any, required []string) registry.Tool {
	return registry.Tool{
		Name:        name,
		Description: description,
		InputSchema: registry.MakeToolInputSchema(properties, required, nil),
	}
}

func TestToolToSkill_BasicMapping(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	td := registry.ToolDefinition{
		Tool: newTestTool("systemd_status", "Get systemd unit status",
			map[string]any{"unit": map[string]any{"type": "string", "description": "Unit name"}},
			[]string{"unit"},
		),
		Category: "system",
		Tags:     []string{"systemd", "monitoring"},
		IsWrite:  false,
	}

	skill := tr.ToolToSkill(td)

	if skill.ID != "systemd_status" {
		t.Errorf("skill.ID = %q, want %q", skill.ID, "systemd_status")
	}
	if skill.Name != "systemd_status" {
		t.Errorf("skill.Name = %q, want %q", skill.Name, "systemd_status")
	}
	if skill.Description != "Get systemd unit status" {
		t.Errorf("skill.Description = %q, want %q", skill.Description, "Get systemd unit status")
	}

	// Check input modes.
	if len(skill.InputModes) != 1 || skill.InputModes[0] != "application/json" {
		t.Errorf("skill.InputModes = %v, want [application/json]", skill.InputModes)
	}
}

func TestToolToSkill_Tags(t *testing.T) {
	t.Parallel()

	tr := &Translator{SkillTags: []string{"mcpkit"}}
	td := registry.ToolDefinition{
		Tool:     newTestTool("docker_restart", "Restart a Docker container", nil, nil),
		Category: "containers",
		Tags:     []string{"docker"},
		IsWrite:  true,
	}

	skill := tr.ToolToSkill(td)

	// Expected tags: category, tool tags, write, default skill tags.
	wantTags := []string{"containers", "docker", "write", "mcpkit"}
	if len(skill.Tags) != len(wantTags) {
		t.Fatalf("len(skill.Tags) = %d, want %d; got %v", len(skill.Tags), len(wantTags), skill.Tags)
	}
	for i, tag := range wantTags {
		if skill.Tags[i] != tag {
			t.Errorf("skill.Tags[%d] = %q, want %q", i, skill.Tags[i], tag)
		}
	}
}

func TestToolToSkill_ReadTag(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	td := registry.ToolDefinition{
		Tool:    newTestTool("list_files", "List files", nil, nil),
		IsWrite: false,
	}

	skill := tr.ToolToSkill(td)

	found := false
	for _, tag := range skill.Tags {
		if tag == "read" {
			found = true
		}
		if tag == "write" {
			t.Error("unexpected 'write' tag on read-only tool")
		}
	}
	if !found {
		t.Error("missing 'read' tag on read-only tool")
	}
}

func TestToolToSkill_InputSchemaInExamples(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	td := registry.ToolDefinition{
		Tool: newTestTool("greet", "Say hello",
			map[string]any{"name": map[string]any{"type": "string", "description": "Who to greet"}},
			[]string{"name"},
		),
	}

	skill := tr.ToolToSkill(td)

	if len(skill.Examples) == 0 {
		t.Fatal("expected non-empty Examples with embedded input schema")
	}
	// The example should contain valid JSON with "type" and "properties".
	example := skill.Examples[0]
	if example == "" {
		t.Error("empty example string")
	}
}

func TestToolToSkill_EmptyTool(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	td := registry.ToolDefinition{
		Tool: newTestTool("noop", "", nil, nil),
	}

	skill := tr.ToolToSkill(td)

	if skill.ID != "noop" {
		t.Errorf("skill.ID = %q, want %q", skill.ID, "noop")
	}
	// Should still have the read tag.
	if len(skill.Tags) == 0 {
		t.Error("expected at least a read tag")
	}
}

// --- CallResultToArtifact tests ---

func TestCallResultToArtifact_TextContent(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeTextContent("hello world")},
	}

	artifact := tr.CallResultToArtifact(result)

	if len(artifact.Parts) != 1 {
		t.Fatalf("len(artifact.Parts) = %d, want 1", len(artifact.Parts))
	}
	if artifact.Parts[0].Text() != "hello world" {
		t.Errorf("text = %q, want %q", artifact.Parts[0].Text(), "hello world")
	}
}

func TestCallResultToArtifact_ImageContent(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeImageContent([]byte("fake-png-bytes"), "image/png")},
	}

	artifact := tr.CallResultToArtifact(result)

	if len(artifact.Parts) != 1 {
		t.Fatalf("len(artifact.Parts) = %d, want 1", len(artifact.Parts))
	}

	part := artifact.Parts[0]
	raw := part.Raw()
	if raw == nil {
		t.Fatal("expected raw part for image content")
	}
	if string(raw) != "fake-png-bytes" {
		t.Errorf("raw = %q, want %q", string(raw), "fake-png-bytes")
	}
	if part.MediaType != "image/png" {
		t.Errorf("mediaType = %q, want %q", part.MediaType, "image/png")
	}
}

func TestCallResultToArtifact_MultipleContent(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{
			registry.MakeTextContent("line 1"),
			registry.MakeTextContent("line 2"),
		},
	}

	artifact := tr.CallResultToArtifact(result)

	if len(artifact.Parts) != 2 {
		t.Fatalf("len(artifact.Parts) = %d, want 2", len(artifact.Parts))
	}
}

func TestCallResultToArtifact_ErrorResult(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeTextContent("something went wrong")},
		IsError: true,
	}

	artifact := tr.CallResultToArtifact(result)

	if artifact.Description != "error" {
		t.Errorf("description = %q, want %q", artifact.Description, "error")
	}
	if len(artifact.Parts) != 1 {
		t.Fatalf("len(artifact.Parts) = %d, want 1", len(artifact.Parts))
	}
	if artifact.Parts[0].Text() != "something went wrong" {
		t.Errorf("text = %q, want %q", artifact.Parts[0].Text(), "something went wrong")
	}
}

func TestCallResultToArtifact_NilResult(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	artifact := tr.CallResultToArtifact(nil)

	if artifact.Parts == nil {
		t.Error("expected non-nil parts for nil result")
	}
	if len(artifact.Parts) != 0 {
		t.Errorf("len(artifact.Parts) = %d, want 0", len(artifact.Parts))
	}
}

func TestCallResultToArtifact_EmptyContent(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{},
	}

	artifact := tr.CallResultToArtifact(result)

	if len(artifact.Parts) != 0 {
		t.Errorf("len(artifact.Parts) = %d, want 0", len(artifact.Parts))
	}
}

// --- ErrorToTaskStatus tests ---

func TestErrorToTaskStatus_InvalidParam(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrInvalidParam, "missing field 'unit'")

	if status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateFailed)
	}
	if status.Message == nil {
		t.Fatal("expected non-nil status message")
	}
	if len(status.Message.Parts) == 0 {
		t.Fatal("expected non-empty message parts")
	}
	text := status.Message.Parts[0].Text()
	if text == "" {
		t.Error("expected non-empty error text")
	}
}

func TestErrorToTaskStatus_NotFound(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrNotFound, "tool not found")

	if status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateFailed)
	}
}

func TestErrorToTaskStatus_PermissionDenied(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrPermission, "access denied")

	if status.State != a2atypes.TaskStateRejected {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateRejected)
	}
}

func TestErrorToTaskStatus_Timeout(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrTimeout, "tool timed out")

	if status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateFailed)
	}
}

func TestErrorToTaskStatus_RateLimited(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrRateLimited, "rate limit exceeded")

	if status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateFailed)
	}
}

func TestErrorToTaskStatus_Internal(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	status := tr.ErrorToTaskStatus(handler.ErrInternal, "unexpected error")

	if status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", status.State, a2atypes.TaskStateFailed)
	}
}

// --- MessageToCallToolRequest tests ---

func TestMessageToCallToolRequest_DataPart(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewDataPart(map[string]any{
				"skill": "systemd_status",
				"arguments": map[string]any{
					"unit": "docker.service",
				},
			}),
		},
	}

	name, args, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "systemd_status" {
		t.Errorf("name = %q, want %q", name, "systemd_status")
	}
	if args["unit"] != "docker.service" {
		t.Errorf("args[unit] = %v, want %q", args["unit"], "docker.service")
	}
}

func TestMessageToCallToolRequest_DataPartNoArguments(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewDataPart(map[string]any{
				"skill": "list_all",
			}),
		},
	}

	name, args, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "list_all" {
		t.Errorf("name = %q, want %q", name, "list_all")
	}
	if args == nil {
		t.Error("expected non-nil args map")
	}
}

func TestMessageToCallToolRequest_TextPartFallback(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewTextPart(`{"unit": "nginx.service"}`),
		},
	}
	skill := a2atypes.AgentSkill{ID: "systemd_status"}

	name, args, err := tr.MessageToCallToolRequest(msg, skill)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "systemd_status" {
		t.Errorf("name = %q, want %q", name, "systemd_status")
	}
	if args["unit"] != "nginx.service" {
		t.Errorf("args[unit] = %v, want %q", args["unit"], "nginx.service")
	}
}

func TestMessageToCallToolRequest_PlainTextFallback(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewTextPart("restart docker please"),
		},
	}
	skill := a2atypes.AgentSkill{ID: "generic_action"}

	name, args, err := tr.MessageToCallToolRequest(msg, skill)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "generic_action" {
		t.Errorf("name = %q, want %q", name, "generic_action")
	}
	if args["input"] != "restart docker please" {
		t.Errorf("args[input] = %v, want %q", args["input"], "restart docker please")
	}
}

func TestMessageToCallToolRequest_NoSkillID(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewTextPart("hello"),
		},
	}

	_, _, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err == nil {
		t.Error("expected error for message with no skill identifier")
	}
}

func TestMessageToCallToolRequest_EmptyMessage(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role:  a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{},
	}

	_, _, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestMessageToCallToolRequest_NilParts(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
	}

	_, _, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err == nil {
		t.Error("expected error for nil parts")
	}
}

func TestMessageToCallToolRequest_SkillHintEmptyParts(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role:  a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{},
	}
	skill := a2atypes.AgentSkill{ID: "some_tool"}

	name, args, err := tr.MessageToCallToolRequest(msg, skill)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "some_tool" {
		t.Errorf("name = %q, want %q", name, "some_tool")
	}
	if len(args) != 0 {
		t.Errorf("expected empty args, got %v", args)
	}
}

func TestMessageToCallToolRequest_NilPartInSlice(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			nil,
			a2atypes.NewDataPart(map[string]any{
				"skill":     "test_tool",
				"arguments": map[string]any{"key": "value"},
			}),
		},
	}

	name, args, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test_tool" {
		t.Errorf("name = %q, want %q", name, "test_tool")
	}
	if args["key"] != "value" {
		t.Errorf("args[key] = %v, want %q", args["key"], "value")
	}
}

// --- CallResultToEvents tests ---

func taskInfo() a2atypes.TaskInfo {
	return a2atypes.TaskInfo{ContextID: "ctx-1", TaskID: "task-1"}
}

func collectTranslatorEvents(tr *Translator, info a2atypes.TaskInfo, result *registry.CallToolResult, err error) []a2atypes.Event {
	var events []a2atypes.Event
	for ev, evErr := range tr.CallResultToEvents(info, result, err) {
		if evErr != nil {
			break
		}
		events = append(events, ev)
	}
	return events
}

func TestCallResultToEvents_Success(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeTextContent("all good")},
	}

	events := collectTranslatorEvents(tr, taskInfo(), result, nil)

	if len(events) != 2 {
		t.Fatalf("expected 2 events (artifact + completed), got %d", len(events))
	}
	if _, ok := events[0].(*a2atypes.TaskArtifactUpdateEvent); !ok {
		t.Errorf("event[0] type = %T, want *TaskArtifactUpdateEvent", events[0])
	}
	statusEv, ok := events[1].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Errorf("event[1] type = %T, want *TaskStatusUpdateEvent", events[1])
	} else if statusEv.Status.State != a2atypes.TaskStateCompleted {
		t.Errorf("state = %q, want %q", statusEv.Status.State, a2atypes.TaskStateCompleted)
	}
}

func TestCallResultToEvents_HandlerError(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	events := collectTranslatorEvents(tr, taskInfo(), nil, errors.New("context deadline exceeded"))

	if len(events) != 1 {
		t.Fatalf("expected 1 event (failed status), got %d", len(events))
	}
	statusEv, ok := events[0].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Errorf("event[0] type = %T, want *TaskStatusUpdateEvent", events[0])
	} else if statusEv.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", statusEv.Status.State, a2atypes.TaskStateFailed)
	}
}

func TestCallResultToEvents_ErrorResult(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeTextContent("invalid param")},
		IsError: true,
	}

	events := collectTranslatorEvents(tr, taskInfo(), result, nil)

	if len(events) != 1 {
		t.Fatalf("expected 1 event (failed status), got %d", len(events))
	}
	statusEv, ok := events[0].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Errorf("event[0] type = %T, want *TaskStatusUpdateEvent", events[0])
	} else if statusEv.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("state = %q, want %q", statusEv.Status.State, a2atypes.TaskStateFailed)
	}
}

func TestCallResultToEvents_ErrorResultNoContent(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{IsError: true}

	events := collectTranslatorEvents(tr, taskInfo(), result, nil)

	if len(events) != 1 {
		t.Fatalf("expected 1 failed-status event, got %d", len(events))
	}
}

// --- BuildCallToolRequest tests ---

func TestBuildCallToolRequest_Basic(t *testing.T) {
	t.Parallel()

	req := BuildCallToolRequest("my_tool", map[string]any{"x": 1})

	if req.Params.Name != "my_tool" {
		t.Errorf("name = %q, want %q", req.Params.Name, "my_tool")
	}
	args := registry.ExtractArguments(req)
	if args == nil {
		t.Fatal("expected non-nil arguments")
	}
	// JSON round-tripping (the official_sdk build's arguments path) turns
	// an int into a float64; compare loosely.
	switch v := args["x"].(type) {
	case int:
		if v != 1 {
			t.Errorf("args[x] = %v, want 1", v)
		}
	case float64:
		if v != 1 {
			t.Errorf("args[x] = %v, want 1", v)
		}
	default:
		t.Errorf("args[x] type = %T, want int or float64", args["x"])
	}
}

func TestBuildCallToolRequest_NilArgs(t *testing.T) {
	t.Parallel()

	req := BuildCallToolRequest("noop", nil)

	if req.Params.Name != "noop" {
		t.Errorf("name = %q, want %q", req.Params.Name, "noop")
	}
}

// --- contentToPart edge case tests ---

func TestContentToPart_NilContent(t *testing.T) {
	t.Parallel()

	part := contentToPart(nil)
	if part != nil {
		t.Errorf("expected nil part for nil content, got %v", part)
	}
}

func TestContentToPart_EmbeddedResource(t *testing.T) {
	t.Parallel()

	content := registry.MakeEmbeddedResourceText("file:///tmp/test.txt", "text/plain", "resource content")

	part := contentToPart(content)
	if part == nil {
		t.Fatal("expected non-nil part for EmbeddedResource")
	}
	// EmbeddedResource maps to DataPart.
	data := part.Data()
	if data == nil {
		t.Fatal("expected DataPart with non-nil data for EmbeddedResource")
	}
}

// --- toStringMap tests ---

func TestToStringMap_ValidMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{"key": "value"}
	result, ok := toStringMap(input)
	if !ok {
		t.Error("expected ok = true for map[string]any")
	}
	if result["key"] != "value" {
		t.Errorf("expected key = %q, got %v", "value", result["key"])
	}
}

func TestToStringMap_InvalidType(t *testing.T) {
	t.Parallel()

	// String is not a map.
	_, ok := toStringMap("not a map")
	if ok {
		t.Error("expected ok = false for string input")
	}

	// Slice is not a map.
	_, ok = toStringMap([]string{"a", "b"})
	if ok {
		t.Error("expected ok = false for slice input")
	}

	// Int is not a map.
	_, ok = toStringMap(42)
	if ok {
		t.Error("expected ok = false for int input")
	}

	// nil is not a map.
	_, ok = toStringMap(nil)
	if ok {
		t.Error("expected ok = false for nil input")
	}
}

// --- marshalInputSchema edge case tests ---

func TestMarshalInputSchema_WithAdditionalProperties(t *testing.T) {
	t.Parallel()

	schema := registry.MakeToolInputSchema(
		map[string]any{"name": map[string]any{"type": "string"}},
		[]string{"name"},
		false,
	)

	result := marshalInputSchema(schema)
	if result == "" {
		t.Fatal("expected non-empty schema JSON")
	}
	// Should contain additionalProperties.
	if !containsSubstr(result, "additionalProperties") {
		t.Errorf("expected schema to contain 'additionalProperties', got %s", result)
	}
}

func TestMarshalInputSchema_EmptySchema(t *testing.T) {
	t.Parallel()

	schema := registry.MakeToolInputSchema(nil, nil, nil)
	result := marshalInputSchema(schema)
	if result == "" {
		t.Fatal("expected non-empty schema JSON even for empty schema")
	}
}

// --- MessageToCallToolRequest edge cases ---

func TestMessageToCallToolRequest_DataPartWithNonMapData(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	// DataPart with a string instead of a map — should be skipped.
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewDataPart("not a map"),
			a2atypes.NewDataPart(map[string]any{
				"skill":     "actual_tool",
				"arguments": map[string]any{"x": 1},
			}),
		},
	}

	name, args, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "actual_tool" {
		t.Errorf("name = %q, want %q", name, "actual_tool")
	}
	if args["x"] != 1 {
		t.Errorf("args[x] = %v, want 1", args["x"])
	}
}

func TestMessageToCallToolRequest_DataPartEmptySkillID(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	// DataPart with an empty skill field — should be skipped.
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewDataPart(map[string]any{
				"skill":     "",
				"arguments": map[string]any{"x": 1},
			}),
		},
	}

	_, _, err := tr.MessageToCallToolRequest(msg, a2atypes.AgentSkill{})
	if err == nil {
		t.Error("expected error for empty skill ID without fallback")
	}
}

func TestMessageToCallToolRequest_SkillHintWithNilParts(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	msg := a2atypes.Message{
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			nil,
			nil,
		},
	}
	skill := a2atypes.AgentSkill{ID: "fallback_tool"}

	name, args, err := tr.MessageToCallToolRequest(msg, skill)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "fallback_tool" {
		t.Errorf("name = %q, want %q", name, "fallback_tool")
	}
	if len(args) != 0 {
		t.Errorf("expected empty args, got %v", args)
	}
}

// --- CallResultToEvents edge cases ---

func TestCallResultToEvents_YieldAbortOnArtifact(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	result := &registry.CallToolResult{
		Content: []registry.Content{registry.MakeTextContent("result")},
	}

	// Collect only the first event, then abort.
	var events []a2atypes.Event
	for ev, err := range tr.CallResultToEvents(taskInfo(), result, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
		break
	}

	// Should only have the artifact event, not the completed status.
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*a2atypes.TaskArtifactUpdateEvent); !ok {
		t.Errorf("expected artifact event, got %T", events[0])
	}
}

func TestCallResultToEvents_NilResultWithError(t *testing.T) {
	t.Parallel()

	tr := &Translator{}
	events := collectTranslatorEvents(tr, taskInfo(), nil, errors.New("boom"))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	statusEv, ok := events[0].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected status update, got %T", events[0])
	}
	if statusEv.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed, got %s", statusEv.Status.State)
	}
}

// --- helper ---

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
