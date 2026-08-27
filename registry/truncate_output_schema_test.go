package registry

import (
	"context"
	"strings"
	"testing"
)

// The defect these tests pin, in the client's own words.
//
// Claude Code 2.1.247's bundled MCP client (verified by string-searching the
// live binary at ~/.local/share/claude/versions/2.1.247, 2026-08-27) does
// this on every tools/call response for a tool that declares an outputSchema:
//
//	if (!result.structuredContent && !result.isError)
//	    throw new Error(`Tool ${name} has an output schema but did not return structured content`)
//	if (result.structuredContent) { ...validate against the schema... }
//
// Two facts follow, and together they decide what an over-cap result may be:
//
//   - A SUCCESS-shaped result with structuredContent dropped is FATAL, not
//     degraded. The caller gets a schema complaint that says nothing about
//     size, and the corrective hint mcpkit writes into content[0].text never
//     reaches the model because the whole result is rejected first.
//   - An ERROR-shaped result with no structuredContent is accepted (the
//     `&& !isError` guard), and its text is the only channel an error has, so
//     the text IS what the model receives.
//
// Note the second branch has no `!isError` guard: any structuredContent
// present is validated against the schema even on an error result. So
// attaching a generic {"error": ...} object to the refusal would trade the
// original schema complaint for a different one — which is why the fix
// returns an error result carrying NO structuredContent rather than a
// synthesised one.
//
// Live trace this reproduces: two secretstudios_opnsense_device_inventory_
// snapshot calls with {"limit":200} died this way at 02:01:43Z/02:01:44Z on
// 2026-08-27; the server logged both as outcome=ok, and the agent then burned
// 11 further MCP calls and 3 tool searches before falling back to raw shell.

// schemaToolParams are the input properties the fixture tool declares. They
// are deliberately shaped like a real paginated tool's: some can narrow the
// response (limit, offset, log_limit) and some cannot (site, full).
var schemaToolParams = map[string]any{
	"site":      map[string]any{"type": "string"},
	"full":      map[string]any{"type": "boolean"},
	"limit":     map[string]any{"type": "integer"},
	"offset":    map[string]any{"type": "integer"},
	"log_limit": map[string]any{"type": "integer"},
}

// newSchemaDeclaringTool builds a tool that declares BOTH an input schema and
// an output schema, the shape every one of secretstudios-mcp's 369 tools has
// on the wire (measured 2026-08-27 via a held-open stdio tools/list probe:
// 369/369 carry an outputSchema).
func newSchemaDeclaringTool(name string, handler ToolHandlerFunc) ToolDefinition {
	td := newTestTool(name, "test", handler)
	td.Tool.InputSchema = MakeToolInputSchema(schemaToolParams, nil, nil)
	td.OutputSchema = testOutputSchema()
	return td
}

// runOversizeSchemaTool registers one over-cap schema-declaring tool and
// returns what the wrapped handler hands back to the client.
func runOversizeSchemaTool(t *testing.T, name string, maxSize int, td ToolDefinition) *CallToolResult {
	t.Helper()
	r := NewToolRegistry(Config{MaxResponseSize: maxSize})
	r.RegisterModule(&testModule{name: "test", tools: []ToolDefinition{td}})

	registered, ok := r.GetTool(name)
	if !ok {
		t.Fatalf("%s not registered", name)
	}
	result, err := r.wrapHandler(name, registered)(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	return result
}

// TestOverCapResultFromSchemaDeclaringToolIsNotSilentlyFatal is the
// reproduction of the defect. It asserts the client contract directly rather
// than any particular implementation: a tool that declares an outputSchema
// must never hand back a success-shaped result with structuredContent
// missing, because that combination is the one the client throws on.
func TestOverCapResultFromSchemaDeclaringToolIsNotSilentlyFatal(t *testing.T) {
	const maxSize = 4096

	data, indented := bigStructuredPayload(200)
	result := runOversizeSchemaTool(t, "oversize_schema_tool", maxSize,
		newSchemaDeclaringTool("oversize_schema_tool", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeStructuredResult(MakeTextContent(indented), data), nil
		}))

	// Precondition, asserted not assumed: the fixture must genuinely be over
	// the cap, or everything below is vacuous.
	if len(indented) <= maxSize {
		t.Fatalf("fixture is not over budget: %d bytes against a %d cap — the guard cannot fire", len(indented), maxSize)
	}

	textBytes, structBytes, _ := resultSizes(t, result)

	if !IsResultError(result) && result.StructuredContent == nil {
		t.Fatalf("over-cap result came back success-shaped with no structuredContent (%d bytes of text). "+
			"A client that validates against this tool's declared outputSchema rejects that outright "+
			"(\"has an output schema but did not return structured content\"), so the caller sees a schema "+
			"complaint instead of a size problem and the truncation hint in the text is discarded with it.",
			textBytes)
	}

	// Whichever shape it took, it must still respect the cap it exists to
	// enforce — a "fix" that simply stops truncating would fail here.
	if total := textBytes + structBytes; total > maxSize+truncationMarkerAllowance {
		t.Errorf("cap bypassed: over-cap result totals %d bytes (text %d + structuredContent %d) against max %d (+%d marker allowance)",
			total, textBytes, structBytes, maxSize, truncationMarkerAllowance)
	}
}

// TestOverCapGuidanceNamesThisToolsOwnParameters pins the second half of the
// defect: the corrective hint has to reach the model, and it has to be
// actionable. "Narrow the request" with no parameter names is not actionable,
// and hardcoded "limit/offset/filters" is a guess that may name nothing the
// tool accepts.
func TestOverCapGuidanceNamesThisToolsOwnParameters(t *testing.T) {
	const maxSize = 4096

	data, indented := bigStructuredPayload(200)
	result := runOversizeSchemaTool(t, "oversize_hint_tool", maxSize,
		newSchemaDeclaringTool("oversize_hint_tool", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeStructuredResult(MakeTextContent(indented), data), nil
		}))

	// The channel the model actually receives. For an error result that is
	// the text; a success-shaped result with structuredContent would have to
	// carry the guidance there instead, and this assertion would need
	// revisiting alongside that change.
	if !IsResultError(result) {
		t.Fatalf("over-cap result is not an error result, so its text is not the channel the model reads; got structuredContent=%v", result.StructuredContent != nil)
	}
	if result.StructuredContent != nil {
		t.Error("the refusal carries structuredContent: the client validates any structuredContent against the tool's schema even on an error result, so this trades one schema complaint for another")
	}

	var text string
	for _, c := range result.Content {
		if txt, ok := ExtractTextContent(c); ok {
			text += txt
		}
	}
	if text == "" {
		t.Fatal("over-cap error result carries no text at all: nothing reaches the model")
	}

	for _, want := range []string{"limit", "offset", "log_limit"} {
		if !strings.Contains(text, want) {
			t.Errorf("guidance does not name this tool's %q parameter; got:\n%s", want, text)
		}
	}
	// A parameter the tool does NOT accept must not be invented. "filters"
	// is the wording the pre-fix hardcoded hint used.
	if strings.Contains(text, "filters") {
		t.Errorf("guidance names a %q parameter this tool does not declare; got:\n%s", "filters", text)
	}
}

// TestOverCapResultIsVisibleToMiddleware is the telemetry half of the defect.
// The over-cap failure was invisible to secretstudios-mcp's invocation ledger
// (both live device_inventory_snapshot deaths recorded outcome=ok) for a
// structural reason: truncation ran OUTSIDE the whole configured middleware
// chain, so the recorder measured the handler's intact result and never saw
// what the client was actually handed.
//
// This test drives that through a real middleware, the same position a
// telemetry recorder occupies.
func TestOverCapResultIsVisibleToMiddleware(t *testing.T) {
	const maxSize = 4096

	var observedError, observed bool
	spy := func(_ string, _ ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			result, err := next(ctx, req)
			observed = true
			observedError = err != nil || IsResultError(result)
			return result, err
		}
	}

	data, indented := bigStructuredPayload(200)
	r := NewToolRegistry(Config{MaxResponseSize: maxSize, Middleware: []Middleware{spy}})
	r.RegisterModule(&testModule{name: "test", tools: []ToolDefinition{
		newSchemaDeclaringTool("oversize_observed_tool", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeStructuredResult(MakeTextContent(indented), data), nil
		}),
	}})

	td, ok := r.GetTool("oversize_observed_tool")
	if !ok {
		t.Fatal("oversize_observed_tool not registered")
	}
	if _, err := r.wrapHandler("oversize_observed_tool", td)(context.Background(), makeEmptyCallToolRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !observed {
		t.Fatal("middleware never ran: the fixture proves nothing")
	}
	if !observedError {
		t.Error("a middleware in the chain saw a clean success for a call the client cannot use. " +
			"A telemetry recorder in this position records outcome=ok and the failure stays invisible.")
	}
}

// TestOverCapDetectedViaCopiedToolOutputSchema covers the other place the
// output schema can live. ApplyToolMetadata copies ToolDefinition.OutputSchema
// into Tool.OutputSchema; a consumer that pre-applies it (or sets Tool
// .OutputSchema directly) must be recognised just the same, or the fix silently
// does not apply to it.
func TestOverCapDetectedViaCopiedToolOutputSchema(t *testing.T) {
	const maxSize = 4096

	data, indented := bigStructuredPayload(200)
	td := newSchemaDeclaringTool("oversize_copied_schema_tool", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeStructuredResult(MakeTextContent(indented), data), nil
	})
	td = ApplyToolMetadata(td, "", false)
	// Only Tool.OutputSchema remains populated now.
	td.OutputSchema = nil

	result := runOversizeSchemaTool(t, "oversize_copied_schema_tool", maxSize, td)

	if !IsResultError(result) && result.StructuredContent == nil {
		t.Error("a tool whose schema lives on Tool.OutputSchema (the ApplyToolMetadata-copied shape) still got a success-shaped result with no structuredContent")
	}
}

// TestUnderCapSchemaToolKeepsStructuredContent is the negative control. The
// refusal must be reachable ONLY by going over the cap; a normal result from
// a schema-declaring tool has to pass through untouched, with both halves
// intact.
func TestUnderCapSchemaToolKeepsStructuredContent(t *testing.T) {
	data, indented := bigStructuredPayload(3)
	result := runOversizeSchemaTool(t, "small_schema_tool", 128*1024,
		newSchemaDeclaringTool("small_schema_tool", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeStructuredResult(MakeTextContent(indented), data), nil
		}))

	if IsResultError(result) {
		t.Error("an under-cap result was turned into an error")
	}
	textBytes, structBytes, marker := resultSizes(t, result)
	if marker {
		t.Error("an under-cap result was marked truncated")
	}
	if structBytes == 0 {
		t.Error("an under-cap result lost its structuredContent")
	}
	if textBytes != len(indented) {
		t.Errorf("under-cap text changed: got %d bytes, want %d", textBytes, len(indented))
	}
}

// TestOverCapToolWithoutOutputSchemaStillDegrades is the other-consumers
// control. mcpkit has consumers beyond secretstudios-mcp, and a tool that
// declares no outputSchema has no client-side validation to fail — for it, a
// clipped-and-marked partial result is a genuine degradation and must stay
// one. Turning every over-cap result into an error would break that contract.
func TestOverCapToolWithoutOutputSchemaStillDegrades(t *testing.T) {
	const maxSize = 4096

	data, indented := bigStructuredPayload(200)
	td := newTestTool("oversize_schemaless_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeStructuredResult(MakeTextContent(indented), data), nil
	})

	result := runOversizeSchemaTool(t, "oversize_schemaless_tool", maxSize, td)

	if IsResultError(result) {
		t.Error("a schemaless tool's over-cap result became an error: mcpkit's other consumers lose a working degradation path")
	}
	textBytes, structBytes, marker := resultSizes(t, result)
	if !marker {
		t.Error("a schemaless tool's over-cap result was truncated with no marker: silent truncation")
	}
	if structBytes != 0 {
		t.Errorf("structuredContent survived a truncation that was announced in the text: %d bytes", structBytes)
	}
	if total := textBytes + structBytes; total > maxSize {
		t.Errorf("degraded result totals %d bytes against a %d cap — the marker reservation did not hold, so a second pass would clip it again",
			total, maxSize)
	}
}
