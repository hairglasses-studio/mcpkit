package observability

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys for MCP tool calls.
// These follow the OpenTelemetry GenAI semantic conventions spec
// (https://opentelemetry.io/docs/specs/semconv/gen-ai/).
const (
	// AttrGenAISystem identifies the AI system. Set to "mcp" for MCP tool calls.
	AttrGenAISystem = attribute.Key("gen_ai.system")

	// AttrGenAIOperationName identifies the operation type. Set to "tool_call" for tool invocations.
	AttrGenAIOperationName = attribute.Key("gen_ai.operation.name")

	// AttrGenAIRequestModel identifies the model name used for the request.
	AttrGenAIRequestModel = attribute.Key("gen_ai.request.model")

	// AttrGenAIUsageInput records the number of input tokens consumed.
	AttrGenAIUsageInput = attribute.Key("gen_ai.usage.input_tokens")

	// AttrGenAIUsageOutput records the number of output tokens produced.
	AttrGenAIUsageOutput = attribute.Key("gen_ai.usage.output_tokens")
)
