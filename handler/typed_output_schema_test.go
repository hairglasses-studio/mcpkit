package handler

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// typedOutputSchemaTestOutput mirrors the shape TypedHandler-built tools
// commonly return (see internal/opnsense's listOutput[T] pattern in
// secretstudios-mcp): a count plus a nested items list.
type typedOutputSchemaTestOutput struct {
	Count int      `json:"count"`
	Items []string `json:"items"`
}

type typedOutputSchemaTestInput struct {
	Query string `json:"query"`
}

// TestTypedHandler_OutputSchema verifies the P52.6/P52.7 round 6 fix:
// typed_official.go's TypedHandler previously never populated
// Tool.OutputSchema at all (only typed.go, the mcp-go side, called
// generateOutputSchema[Out]()) -- every TypedHandler tool on the official
// build silently advertised no output schema. Written entirely against the
// SDK-neutral registry.OutputSchemaType/OutputSchemaProperties accessors
// (also new this round) so the same test runs unmodified under both tags.
func TestTypedHandler_OutputSchema(t *testing.T) {
	td := TypedHandler[typedOutputSchemaTestInput, typedOutputSchemaTestOutput](
		"test_schema_tool",
		"a tool for output-schema testing",
		func(_ context.Context, _ typedOutputSchemaTestInput) (typedOutputSchemaTestOutput, error) {
			return typedOutputSchemaTestOutput{}, nil
		},
	)

	// TypedHandler sets ToolDefinition.OutputSchema directly; ApplyToolMetadata
	// (portable, registry/metadata.go) is what real registration paths use to
	// copy it onto Tool.OutputSchema -- exercise that same path rather than
	// asserting on the ToolDefinition-level field, matching how real consumer
	// tests (e.g. secretstudios-mcp's dhcp_static_test.go) read it back.
	annotated := registry.ApplyToolMetadata(td, "", false)

	typ, ok := registry.OutputSchemaType(annotated.Tool)
	if !ok {
		t.Fatal("OutputSchemaType: not declared, want object")
	}
	if typ != "object" {
		t.Errorf("OutputSchemaType = %q, want object", typ)
	}

	props, ok := registry.OutputSchemaProperties(annotated.Tool)
	if !ok {
		t.Fatal("OutputSchemaProperties: not declared, want non-empty")
	}
	for _, field := range []string{"count", "items"} {
		if _, ok := props[field]; !ok {
			t.Errorf("OutputSchemaProperties missing %q; got %+v", field, props)
		}
	}
}
