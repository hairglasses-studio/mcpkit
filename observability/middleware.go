package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Middleware returns a registry.Middleware that records OTel metrics and
// tracing spans for every tool invocation.
//
// It automatically extracts trace context from the MCP _meta field if present,
// enabling cross-server distributed tracing.
//
// If finops.WithTokenUsage has been called on the context by an upstream
// middleware (e.g. the finops middleware), the span will also carry
// gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, and
// gen_ai.request.model attributes per the GenAI semantic conventions.
func (p *Provider) Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		category := td.Category
		if category == "" {
			category = "unknown"
		}
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Extract trace context from request _meta if available
			ctx = p.ExtractMeta(ctx, request.Params.Meta)

			ctx, span := p.StartSpan(ctx, name)
			if span != nil {
				defer span.End()
			}

			p.StartToolExecution(ctx, name, category)
			defer p.EndToolExecution(ctx, name, category)

			// Place a mutable holder on context so inner finops middleware can
			// write token usage back through the immutable context boundary.
			var holder finops.TokenUsageHolder
			ctx = finops.WithTokenUsageHolder(ctx, &holder)

			start := time.Now()
			result, err := next(ctx, request)
			p.RecordToolInvocation(ctx, name, category, time.Since(start), err)

			// Inject trace context into result _meta for the caller
			if result != nil {
				if result.Meta == nil {
					result.Meta = &registry.ToolMeta{}
				}
				p.InjectMeta(ctx, result.Meta)
			}

			// Bridge finops token usage onto the span when available.
			// Prefer the holder (populated by inner finops middleware) over
			// static context values (which require an outer caller).
			if span != nil && span.IsRecording() {
				usage, ok := holder.Load()
				if !ok {
					usage, ok = finops.TokenUsageFromContext(ctx)
				}
				if ok {
					attrs := []attribute.KeyValue{
						AttrGenAIUsageInput.Int(usage.InputTokens),
						AttrGenAIUsageOutput.Int(usage.OutputTokens),
					}
					if usage.Model != "" {
						attrs = append(attrs, AttrGenAIRequestModel.String(usage.Model))
					}
					span.SetAttributes(attrs...)
				}
			}

			return result, err
		}
	}
}

// addGenAISpanAttrs is a helper that sets GenAI token usage attributes on a
// span. It is a no-op when span is nil or not recording.
func addGenAISpanAttrs(span trace.Span, usage finops.TokenUsage) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{
		AttrGenAIUsageInput.Int(usage.InputTokens),
		AttrGenAIUsageOutput.Int(usage.OutputTokens),
	}
	if usage.Model != "" {
		attrs = append(attrs, AttrGenAIRequestModel.String(usage.Model))
	}
	span.SetAttributes(attrs...)
}
