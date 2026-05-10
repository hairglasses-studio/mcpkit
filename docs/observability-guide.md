# Observability Guide

mcpkit ships OpenTelemetry instrumentation that's compatible with the OTel **GenAI semantic conventions** (https://opentelemetry.io/docs/specs/semconv/gen-ai/) plus its own `finops` cost-tracking layer. This guide documents how the two compose so production servers emit canonical metrics that any GenAI-aware backend (Datadog, Honeycomb, Grafana Tempo, Anthropic Studio) can ingest.

## Layered model

```
   Tool handler  ─► observability.StartSpan(...)  ─► OTel span
                                  │
                                  └─► finops.Track(...)  ─► budget enforcement + dashboard export
```

- **`observability/`** — OTel spans with canonical GenAI attribute names (`gen_ai.system`, `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`).
- **`finops/`** — token accounting + dollar-cost estimation + budget policies. Subscribes to the same call-context so accounting and tracing share the same identifiers.

## OTel GenAI attributes emitted

Per `observability/attributes.go`, mcpkit defines these typed attribute keys:

| Constant | Wire name | Value |
|---|---|---|
| `AttrGenAISystem` | `gen_ai.system` | `"mcp"` (or model provider name when known) |
| `AttrGenAIOperationName` | `gen_ai.operation.name` | `"tool_call"`, `"resource_read"`, `"prompt_get"`, etc. |
| `AttrGenAIRequestModel` | `gen_ai.request.model` | Provider-specific (e.g., `"claude-opus-4-7"`, `"gpt-4o"`) when set by the handler |
| `AttrGenAIUsageInput` | `gen_ai.usage.input_tokens` | Per-call input token count |
| `AttrGenAIUsageOutput` | `gen_ai.usage.output_tokens` | Per-call output token count |

These are stable and immutable per OTel SemConv 1.27 (latest as of 2026-05-10). Datadog APM v1.37+ and Honeycomb both index them as first-class GenAI signals.

## Finops → OTel bridging

`finops/` tracks the same input/output token counts that go onto the OTel span. The two are wired through the same call context so:

1. A handler invokes `observability.StartSpan(ctx, "tool_call")` → opens span with `gen_ai.system=mcp`, `gen_ai.operation.name=tool_call`.
2. The handler does its work; at completion, it calls `usage.RecordTokens(in, out)` which:
   - Populates `AttrGenAIUsageInput` / `AttrGenAIUsageOutput` on the active span.
   - Calls `finops.Track(ctx, in, out, model)` to update budget windows + cost estimates.
3. On span end, the OTel exporter (`observability/exporter.go`) ships the canonical attributes to the configured backend.

## Example: wiring a tool handler

```go
func myHandler(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
    ctx, span := observability.StartSpan(ctx, "my_tool")
    defer span.End()

    // ... call the model / external service ...
    inputTokens, outputTokens := 1234, 567

    // One call updates both OTel span AND finops accounting.
    usage := observability.RecordUsage(ctx, observability.Usage{
        InputTokens:  inputTokens,
        OutputTokens: outputTokens,
        Model:        "claude-opus-4-7",
    })

    return registry.MakeTextResult(fmt.Sprintf("cost=%.4f", usage.EstimatedDollars)), nil
}
```

## Production deployment

For a server that wants to ingest into Datadog (or any OTLP-compatible backend):

```go
exporter, err := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("api.datadoghq.com:443"),
    otlptracegrpc.WithHeaders(map[string]string{"DD-API-KEY": apiKey}),
)
// pass into observability.WithExporter(exporter) when bootstrapping the server
```

For a server that wants budget enforcement:

```go
import "github.com/hairglasses-studio/mcpkit/finops"

policy := finops.NewBudgetPolicy(finops.PolicyConfig{
    PerSessionUSD: 5.00,    // halt session if >$5
    PerTenantUSDPerDay: 100, // tenant gate
})
// wrap tool registry with finops middleware
reg.Use(finops.Middleware(policy))
```

The two layers compose: finops enforces budgets *and* the same accounting flows into OTel for fleet-wide dashboards.

## Reference

- [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) — canonical attribute names + values
- [Datadog Agent v1.37+ GenAI support](https://docs.datadoghq.com/llm_observability/) — first ingest target for canonical OTel GenAI spans
- mcpkit `observability/example_test.go` — runnable example
- mcpkit `finops/example_test.go` — budget-policy example

## Quarterly refresh

OTel SemConv versions and provider-side attribute additions move fast. Re-audit this guide when:
- A new OTel SemConv version ships (track [github.com/open-telemetry/semantic-conventions](https://github.com/open-telemetry/semantic-conventions/releases))
- Major provider (Anthropic / OpenAI / Google) publishes new instrumentation guidance
- `mcpkit/observability/attributes.go` adds new attribute keys

Last refresh: **2026-05-10** post-v0.6.0 release.
