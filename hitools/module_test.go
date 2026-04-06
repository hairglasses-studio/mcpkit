//go:build !official_sdk

package hitools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/handler"
)

func TestModuleInfo(t *testing.T) {
	m := &Module{}
	if m.Name() != "hitools" {
		t.Errorf("Name() = %q, want %q", m.Name(), "hitools")
	}
	if m.Description() == "" {
		t.Error("Description() should not be empty")
	}
	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("Tools() returned %d tools, want 1", len(tools))
	}
	if tools[0].Tool.Name != "request_human_input" {
		t.Errorf("tool name = %q, want %q", tools[0].Tool.Name, "request_human_input")
	}
	if tools[0].Category != "human-interaction" {
		t.Errorf("category = %q, want %q", tools[0].Category, "human-interaction")
	}
}

func TestBuildElicitSchema_FreeText(t *testing.T) {
	input := RequestInput{
		Question: "What is the capital of France?",
		Format:   FormatFreeText,
	}
	params := buildElicitSchema(input)

	if params.Mode != mcp.ElicitationModeForm {
		t.Errorf("mode = %q, want %q", params.Mode, mcp.ElicitationModeForm)
	}
	if params.Message != "What is the capital of France?" {
		t.Errorf("message = %q", params.Message)
	}

	schema, ok := params.RequestedSchema.(map[string]any)
	if !ok {
		t.Fatal("schema is not a map")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not a map")
	}
	answer, ok := props["answer"].(map[string]any)
	if !ok {
		t.Fatal("answer property not found")
	}
	if answer["type"] != "string" {
		t.Errorf("answer type = %v, want string", answer["type"])
	}
	if answer["enum"] != nil {
		t.Error("free_text should not have enum")
	}
}

func TestBuildElicitSchema_YesNo(t *testing.T) {
	input := RequestInput{
		Question: "Continue?",
		Format:   FormatYesNo,
	}
	params := buildElicitSchema(input)

	schema, ok := params.RequestedSchema.(map[string]any)
	if !ok {
		t.Fatal("schema is not a map")
	}
	props := schema["properties"].(map[string]any)
	answer := props["answer"].(map[string]any)
	if answer["type"] != "boolean" {
		t.Errorf("answer type = %v, want boolean", answer["type"])
	}
}

func TestBuildElicitSchema_MultipleChoice(t *testing.T) {
	input := RequestInput{
		Question: "Pick a color",
		Format:   FormatMultipleChoice,
		Choices:  []string{"red", "green", "blue"},
	}
	params := buildElicitSchema(input)

	schema, ok := params.RequestedSchema.(map[string]any)
	if !ok {
		t.Fatal("schema is not a map")
	}
	props := schema["properties"].(map[string]any)
	answer := props["answer"].(map[string]any)
	if answer["type"] != "string" {
		t.Errorf("answer type = %v, want string", answer["type"])
	}
	enum, ok := answer["enum"].([]string)
	if !ok {
		t.Fatal("enum is not a string slice")
	}
	if len(enum) != 3 {
		t.Errorf("enum length = %d, want 3", len(enum))
	}
	if enum[0] != "red" || enum[1] != "green" || enum[2] != "blue" {
		t.Errorf("enum = %v, want [red green blue]", enum)
	}
}

func TestBuildElicitSchema_WithContext(t *testing.T) {
	input := RequestInput{
		Question: "Proceed?",
		Context:  "We are about to deploy to production.",
		Format:   FormatYesNo,
	}
	params := buildElicitSchema(input)

	want := "Proceed?\n\nContext: We are about to deploy to production."
	if params.Message != want {
		t.Errorf("message = %q, want %q", params.Message, want)
	}
}

func TestHandleRequestHumanInput_NoServer(t *testing.T) {
	// Without a server in context, the handler should return "pending".
	input := RequestInput{
		Question: "What is your name?",
		Format:   FormatFreeText,
	}
	output, err := handleRequestHumanInput(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Status != "pending" {
		t.Errorf("status = %q, want %q", output.Status, "pending")
	}
	if output.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}

func TestHandleRequestHumanInput_MultipleChoiceValidation(t *testing.T) {
	// Multiple choice without choices should fail.
	input := RequestInput{
		Question: "Pick one",
		Format:   FormatMultipleChoice,
		Choices:  nil,
	}
	_, err := handleRequestHumanInput(context.Background(), input)
	if err == nil {
		t.Error("expected error for multiple_choice without choices")
	}
}

func TestExtractAnswer(t *testing.T) {
	tests := []struct {
		name    string
		content any
		want    string
	}{
		{
			name:    "map with answer",
			content: map[string]any{"answer": "Paris"},
			want:    "Paris",
		},
		{
			name:    "map with boolean answer",
			content: map[string]any{"answer": true},
			want:    "true",
		},
		{
			name:    "map without answer key",
			content: map[string]any{"other": "value"},
			want:    "map[other:value]",
		},
		{
			name:    "non-map content",
			content: "raw string",
			want:    "raw string",
		},
		{
			name:    "nil content",
			content: nil,
			want:    "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAnswer(tt.content)
			if got != tt.want {
				t.Errorf("extractAnswer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolDefinitionHasOutputSchema(t *testing.T) {
	m := &Module{}
	tools := m.Tools()
	td := tools[0]

	// TypedHandler should generate an output schema from RequestOutput.
	if td.OutputSchema == nil {
		t.Fatal("OutputSchema should not be nil")
	}
}

func TestToolInputSchemaHasRequiredQuestion(t *testing.T) {
	m := &Module{}
	tools := m.Tools()
	td := tools[0]

	schema := td.Tool.InputSchema
	if schema.Type != "object" {
		t.Errorf("input schema type = %q, want %q", schema.Type, "object")
	}

	// "question" must be required.
	found := false
	for _, r := range schema.Required {
		if r == "question" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("required fields %v should contain 'question'", schema.Required)
	}
}

// Verify the handler package import works (compile-time check for typed handler).
var _ = handler.FormField{}
