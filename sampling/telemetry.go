//go:build !official_sdk

package sampling

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/finops"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const samplingTracerName = "github.com/hairglasses-studio/mcpkit/sampling"

var (
	attrGenAISystem          = attribute.Key("gen_ai.system")
	attrGenAIOperationName   = attribute.Key("gen_ai.operation.name")
	attrGenAIRequestModel    = attribute.Key("gen_ai.request.model")
	attrGenAIUsageInput      = attribute.Key("gen_ai.usage.input_tokens")
	attrGenAIUsageOutput     = attribute.Key("gen_ai.usage.output_tokens")
	attrGenAIResponseStop    = attribute.Key("gen_ai.response.stop_reason")
	attrGenAIBasURL          = attribute.Key("server.address")
	attrGenAIRequestAttempts = attribute.Key("gen_ai.request.attempts")
)

type llmSpanConfig struct {
	System    string
	Operation string
	Model     string
	BaseURL   string
}

func startLLMSpan(ctx context.Context, cfg llmSpanConfig) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attrGenAISystem.String(cfg.System),
		attrGenAIOperationName.String(cfg.Operation),
	}
	if cfg.Model != "" {
		attrs = append(attrs, attrGenAIRequestModel.String(cfg.Model))
	}
	if cfg.BaseURL != "" {
		attrs = append(attrs, attrGenAIBasURL.String(cfg.BaseURL))
	}
	return otel.Tracer(samplingTracerName).Start(
		ctx,
		"llm."+cfg.Operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

func finishLLMSpan(ctx context.Context, span trace.Span, usage finops.TokenUsage, stopReason string, attempts int, err error) {
	if span == nil {
		return
	}
	if usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.Model != "" {
		if usage.Model != "" {
			span.SetAttributes(attrGenAIRequestModel.String(usage.Model))
		}
		span.SetAttributes(
			attrGenAIUsageInput.Int(usage.InputTokens),
			attrGenAIUsageOutput.Int(usage.OutputTokens),
		)
		if holder, ok := finops.TokenUsageHolderFromContext(ctx); ok && holder != nil {
			holder.Store(usage)
		}
	}
	if stopReason != "" {
		span.SetAttributes(attrGenAIResponseStop.String(stopReason))
	}
	if attempts > 0 {
		span.SetAttributes(attrGenAIRequestAttempts.Int(attempts))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
