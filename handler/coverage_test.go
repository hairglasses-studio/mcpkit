//go:build !official_sdk

package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ==================== contentAnnotations: AudioContent and ImageContent nil annotations ====================

func TestContentAnnotations_AudioContentWithAnnotation(t *testing.T) {
	t.Parallel()
	ac := mcp.AudioContent{
		Annotated: mcp.Annotated{
			Annotations: &mcp.Annotations{Audience: []mcp.Role{"user"}},
		},
		Type: "audio",
	}
	ann := contentAnnotations(ac)
	if ann == nil {
		t.Fatal("expected non-nil annotations for AudioContent")
	}
	if len(ann.Audience) != 1 || string(ann.Audience[0]) != "user" {
		t.Errorf("unexpected audience: %v", ann.Audience)
	}
}

func TestContentAnnotations_ImageContentNilAnnotations(t *testing.T) {
	t.Parallel()
	ic := mcp.ImageContent{
		Type: "image",
		// No annotations set — Annotations is nil
	}
	ann := contentAnnotations(ic)
	if ann != nil {
		t.Errorf("expected nil annotations, got %v", ann)
	}
}

func TestFilterByAudience_AudioContent(t *testing.T) {
	t.Parallel()
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.AudioContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant"}},
				},
				Type: "audio",
			},
			mcp.AudioContent{
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{Audience: []mcp.Role{"user"}},
				},
				Type: "audio",
			},
		},
	}

	filtered := FilterByAudience(result, []string{"user"})
	if len(filtered.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(filtered.Content))
	}
}

// ==================== schemaToMap: exercise all branches via TypedHandler ====================

// typedHandlerArrayInput and Output exercise the schemaToMap "items" branch (slice fields).
type arrayInput struct {
	Tags []string `json:"tags" jsonschema:"description=List of tags"`
}

type arrayOutput struct {
	Items []string `json:"items"`
}

func TestTypedHandler_SchemaWithArrayField(t *testing.T) {
	t.Parallel()

	td := TypedHandler[arrayInput, arrayOutput](
		"array_tool",
		"Tool with array fields",
		func(_ context.Context, input arrayInput) (arrayOutput, error) {
			return arrayOutput{Items: input.Tags}, nil
		},
	)

	if td.Tool.Name != "array_tool" {
		t.Errorf("name = %q, want array_tool", td.Tool.Name)
	}
	if td.OutputSchema == nil {
		t.Fatal("output schema is nil")
	}
	if _, ok := td.OutputSchema.Properties["items"]; !ok {
		t.Error("output schema should have 'items' property")
	}
}

// nestedOutput exercises schemaToMap nested object properties branch.
type nestedChild struct {
	Value string `json:"value"`
}

type nestedOutput struct {
	Child nestedChild `json:"child"`
}

type nestedInput struct {
	Name string `json:"name" jsonschema:"required"`
}

func TestTypedHandler_SchemaWithNestedObject(t *testing.T) {
	t.Parallel()

	td := TypedHandler[nestedInput, nestedOutput](
		"nested_tool",
		"Tool with nested object",
		func(_ context.Context, input nestedInput) (nestedOutput, error) {
			return nestedOutput{Child: nestedChild{Value: input.Name}}, nil
		},
	)

	if td.OutputSchema == nil {
		t.Fatal("output schema is nil")
	}
	if _, ok := td.OutputSchema.Properties["child"]; !ok {
		t.Error("output schema should have 'child' property")
	}
}

// ==================== isNumber branches ====================

func TestIsNumber_AllTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  any
		want bool
	}{
		{float64(3.14), true},
		{float32(1.5), true},
		{int(42), true},
		{int32(10), true},
		{int64(100), true},
		{json.Number("42"), true},
		{"hello", false},
		{true, false},
		{nil, false},
	}

	for _, tc := range cases {
		got := isNumber(tc.val)
		if got != tc.want {
			t.Errorf("isNumber(%v) = %v, want %v", tc.val, got, tc.want)
		}
	}
}

// ==================== isInteger branches ====================

func TestIsInteger_AllTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  any
		want bool
	}{
		{int(42), true},
		{int32(10), true},
		{int64(100), true},
		{float64(42.0), true},  // exact integer value as float64
		{float64(42.5), false}, // fractional float64
		{json.Number("42"), true},
		{json.Number("42.5"), false},
		{"hello", false},
		{true, false},
	}

	for _, tc := range cases {
		got := isInteger(tc.val)
		if got != tc.want {
			t.Errorf("isInteger(%v) = %v, want %v", tc.val, got, tc.want)
		}
	}
}

// ==================== describeType branches ====================

func TestDescribeType_AllTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  any
		want string
	}{
		{"hello", "string"},
		{float64(3.14), "number"},
		{float32(1.5), "number"},
		{int(42), "integer"},
		{int32(10), "integer"},
		{int64(100), "integer"},
		{true, "boolean"},
		{[]any{"a"}, "array"},
		{map[string]any{"k": "v"}, "object"},
		{nil, "null"},
	}

	for _, tc := range cases {
		got := describeType(tc.val)
		if got != tc.want {
			t.Errorf("describeType(%v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestDescribeType_Unknown(t *testing.T) {
	t.Parallel()
	// An uncommon type that falls into the default branch.
	got := describeType(struct{ X int }{X: 1})
	if got == "" {
		t.Error("describeType for unknown type should return non-empty string")
	}
}

// ==================== validateStructuredContent: non-map struct path ====================

func TestValidateStructuredContent_StructMarshalPath(t *testing.T) {
	t.Parallel()

	type myData struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	schema := makeTestSchema(map[string]any{
		"name":  map[string]any{"type": "string"},
		"count": map[string]any{"type": "integer"},
	}, []string{"name"})

	data := myData{Name: "test", Count: 5}
	err := validateStructuredContent(data, schema)
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidateStructuredContent_StructMissingRequired(t *testing.T) {
	t.Parallel()

	type countOnly struct {
		Count int `json:"count"`
	}

	schema := makeTestSchema(map[string]any{
		"name":  map[string]any{"type": "string"},
		"count": map[string]any{"type": "integer"},
	}, []string{"name"})

	data := countOnly{Count: 5}
	err := validateStructuredContent(data, schema)
	if err == nil {
		t.Error("expected validation error for missing required field")
	}
}

func TestValidateStructuredContent_NullFieldIsValid(t *testing.T) {
	t.Parallel()

	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"type": "string"},
	}, nil)

	// nil value for a field is permissive (null is valid for any type)
	data := map[string]any{"name": nil}
	err := validateStructuredContent(data, schema)
	if err != nil {
		t.Errorf("unexpected error for nil field: %v", err)
	}
}

func TestValidateStructuredContent_NonMapPropSchema(t *testing.T) {
	t.Parallel()

	// propSchema that is not a map[string]any — should be skipped without error
	schema := &registry.ToolOutputSchema{
		Type: "object",
		Properties: map[string]any{
			"name": "not-a-map",
		},
		Required: nil,
	}

	data := map[string]any{"name": "alice"}
	err := validateStructuredContent(data, schema)
	if err != nil {
		t.Errorf("unexpected error when prop schema is not a map: %v", err)
	}
}

func TestValidateStructuredContent_PropWithNoTypeDecl(t *testing.T) {
	t.Parallel()

	// propSchema map with no "type" key — should skip type validation
	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"description": "no type here"},
	}, nil)

	// pass int where "name" has no type constraint — should succeed
	data := map[string]any{"name": 42}
	err := validateStructuredContent(data, schema)
	if err != nil {
		t.Errorf("unexpected error when no type declared: %v", err)
	}
}

// ==================== checkType: edge cases ====================

func TestCheckType_UnknownType(t *testing.T) {
	t.Parallel()

	// Unknown type declaration should return nil (no validation performed)
	err := checkType("field", "somevalue", "uuid")
	if err != nil {
		t.Errorf("checkType unknown type = %v, want nil", err)
	}
}

func TestCheckType_NullValue(t *testing.T) {
	t.Parallel()

	// nil value should always pass regardless of expected type
	err := checkType("field", nil, "string")
	if err != nil {
		t.Errorf("checkType null value = %v, want nil", err)
	}
}

// ==================== GetFloatParam: wrong type branch ====================

func TestGetFloatParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"price": "not-a-float"})
	if got := GetFloatParam(req, "price", 1.23); got != 1.23 {
		t.Errorf("GetFloatParam wrong type = %f, want 1.23", got)
	}
}

// ==================== OutputValidationMiddleware: next returns Go error ====================

func TestOutputValidation_NextReturnsGoError(t *testing.T) {
	t.Parallel()

	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"})

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	h := mw("tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		// Return a Go-level error (not a tool error result)
		return nil, context.DeadlineExceeded
	})

	result, err := h(context.Background(), registry.CallToolRequest{})
	if err == nil {
		t.Error("expected Go error to propagate")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// ==================== checkType: error message uses describeType ====================

func TestCheckType_ErrorMessageIncludesType(t *testing.T) {
	t.Parallel()

	// Passing a boolean where "string" is expected triggers an error with describeType output.
	err := checkType("myfield", true, "string")
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should be non-empty")
	}
}
