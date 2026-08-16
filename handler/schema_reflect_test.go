// schema_reflect_test.go -- regression guard for secretstudios-mcp P78.38.
//
// This file carries NO build tag on purpose: every assertion below is written
// against the SDK-neutral registry.InputSchema*/OutputSchema* accessors, so
// the SAME test runs unmodified under both `go test ./handler` (mcp-go) and
// `go test -tags official_sdk ./handler`. That is the point -- the bug it
// guards against existed ONLY on the official_sdk side, and a test that ran
// on just one build could never have caught it.
//
// Why the pre-existing handler tests did not catch the original bug: every
// input/output struct they used (typedOutputSchemaTestInput{Query string
// `json:"query"`}, etc.) had NO `omitempty` on any field. The broken
// generator derived properties by marshaling a zero value, which preserves
// exactly the non-omitempty fields -- so those fixtures produced identical
// output before and after. The struct below is deliberately built the other
// way: it is dominated by `omitempty` fields, which is what real tool inputs
// actually look like (secretstudios-mcp's wan_inspect: 24 of 24 optional).
package handler

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// schemaReflectInput mirrors the shape of a real polymorphic tool input:
// mostly-optional fields carrying jsonschema descriptions, one required
// field, and an enum. Under the pre-P78.38 zero-value-marshal generator the
// official_sdk build advertised ONLY "host" here -- the four omitempty
// fields vanished entirely, as did every description and the required list.
type schemaReflectInput struct {
	Host        string `json:"host" jsonschema:"description=Required target host"`
	View        string `json:"view,omitempty" jsonschema:"description=Which view to render,enum=status,enum=metrics"`
	Fresh       bool   `json:"fresh,omitempty" jsonschema:"description=Bypass the cache"`
	Port        int    `json:"port,omitempty" jsonschema:"description=Optional TCP port"`
	IncludeLogs *bool  `json:"include_logs,omitempty" jsonschema:"description=Include log excerpts"`
}

type schemaReflectOutput struct {
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

// TestTypedHandlerInputSchemaIncludesOmitemptyFields is the core guard: an
// `omitempty` field is still a real, callable parameter, and it MUST appear
// in the advertised input schema. When this fails, callers have no
// wire-level way to discover or supply most of a tool's parameters.
func TestTypedHandlerInputSchemaIncludesOmitemptyFields(t *testing.T) {
	td := TypedHandler[schemaReflectInput, schemaReflectOutput](
		"schema_reflect_tool",
		"a tool for input-schema reflection testing",
		func(_ context.Context, _ schemaReflectInput) (schemaReflectOutput, error) {
			return schemaReflectOutput{}, nil
		},
	)

	typ, ok := registry.InputSchemaType(td.Tool.InputSchema)
	if !ok || typ != "object" {
		t.Fatalf("InputSchemaType = %q (ok=%v), want \"object\" -- the official SDK's "+
			"Server.AddTool panics on any other input-schema type", typ, ok)
	}

	props, ok := registry.InputSchemaProperties(td.Tool.InputSchema)
	if !ok {
		t.Fatal("InputSchemaProperties reported no properties at all -- the tool advertises " +
			"an empty schema, so no caller can supply any parameter")
	}

	want := []string{"host", "view", "fresh", "port", "include_logs"}
	for _, field := range want {
		if _, ok := props[field]; !ok {
			t.Errorf("input schema is missing %q. Got %d propert(y/ies): %v. All but "+
				"\"host\" are `omitempty`, which is exactly the class the pre-P78.38 "+
				"zero-value-marshal generator dropped.", field, len(props), keysOf(props))
		}
	}
	if len(props) != len(want) {
		t.Errorf("input schema has %d properties, want %d: %v", len(props), len(want), keysOf(props))
	}
}

// TestTypedHandlerInputSchemaPreservesDescriptions guards the second half of
// the loss: even for the fields that DID survive the old generator, its
// inferFieldSchema() emitted a bare {"type": ...} and threw away every
// jsonschema description, enum and default. Descriptions are the entire
// mechanism by which a model knows what a parameter means.
func TestTypedHandlerInputSchemaPreservesDescriptions(t *testing.T) {
	td := TypedHandler[schemaReflectInput, schemaReflectOutput](
		"schema_reflect_tool_desc", "desc test",
		func(_ context.Context, _ schemaReflectInput) (schemaReflectOutput, error) {
			return schemaReflectOutput{}, nil
		},
	)
	props, ok := registry.InputSchemaProperties(td.Tool.InputSchema)
	if !ok {
		t.Fatal("InputSchemaProperties reported no properties")
	}

	for field, wantDesc := range map[string]string{
		"host":  "Required target host",
		"view":  "Which view to render",
		"fresh": "Bypass the cache",
	} {
		raw, ok := props[field]
		if !ok {
			t.Errorf("property %q absent", field)
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			t.Errorf("property %q is %T, want map[string]any", field, raw)
			continue
		}
		if got, _ := m["description"].(string); got != wantDesc {
			t.Errorf("property %q description = %q, want %q -- jsonschema tag metadata is being dropped", field, got, wantDesc)
		}
	}

	// Type fidelity: the old generator inferred types from a marshaled zero
	// value, so an int came back as JSON number ("number", not "integer") and
	// a *bool came back as null and was mistyped "string".
	if m, ok := props["port"].(map[string]any); ok {
		if got, _ := m["type"].(string); got != "integer" {
			t.Errorf("property \"port\" type = %q, want \"integer\"", got)
		}
	}
	if m, ok := props["include_logs"].(map[string]any); ok {
		if got, _ := m["type"].(string); got != "boolean" {
			t.Errorf("property \"include_logs\" (a *bool) type = %q, want \"boolean\" -- "+
				"inferring from a marshaled zero value read this as null and called it a string", got)
		}
	}

	// Enum values from the jsonschema tag must survive too.
	if m, ok := props["view"].(map[string]any); ok {
		enum, _ := m["enum"].([]any)
		if len(enum) != 2 {
			t.Errorf("property \"view\" enum = %v, want 2 entries (status, metrics)", m["enum"])
		}
	}
}

// TestTypedHandlerInputSchemaRequired guards required-ness, which the old
// generator never emitted at all: it produced only "type" and "properties",
// so every parameter looked optional regardless of its json tag.
func TestTypedHandlerInputSchemaRequired(t *testing.T) {
	td := TypedHandler[schemaReflectInput, schemaReflectOutput](
		"schema_reflect_tool_req", "required test",
		func(_ context.Context, _ schemaReflectInput) (schemaReflectOutput, error) {
			return schemaReflectOutput{}, nil
		},
	)
	req, ok := registry.InputSchemaRequired(td.Tool.InputSchema)
	if !ok {
		t.Fatal("InputSchemaRequired reported nothing -- \"host\" has no `omitempty` and must be required")
	}
	if len(req) != 1 || req[0] != "host" {
		t.Errorf("required = %v, want exactly [host]", req)
	}
}

// TestTypedHandlerOutputSchemaIncludesOmitemptyFields applies the same guard
// to the output schema, which is generated by the same shared reflection and
// was lossy in exactly the same way.
func TestTypedHandlerOutputSchemaIncludesOmitemptyFields(t *testing.T) {
	td := TypedHandler[schemaReflectInput, schemaReflectOutput](
		"schema_reflect_tool_out", "output test",
		func(_ context.Context, _ schemaReflectInput) (schemaReflectOutput, error) {
			return schemaReflectOutput{}, nil
		},
	)
	annotated := registry.ApplyToolMetadata(td, "", false)
	props, ok := registry.OutputSchemaProperties(annotated.Tool)
	if !ok {
		t.Fatal("OutputSchemaProperties reported nothing")
	}
	for _, field := range []string{"status", "summary"} {
		if _, ok := props[field]; !ok {
			t.Errorf("output schema missing %q (got %v); \"summary\" is `omitempty`", field, keysOf(props))
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
