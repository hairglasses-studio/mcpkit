// Package sampling provides a client interface, context helpers, and request
// builders for requesting LLM completions through the MCP sampling protocol.
//
// [SamplingClient] is the interface tool handlers use to issue
// CreateMessage requests; implementations are injected via
// [WithSamplingClient] and retrieved with [ClientFromContext].
// [InjectMiddleware] wraps a [registry.Middleware] that automatically
// populates the context from the MCP server's connected client, so tools
// receive a sampling client with no additional wiring. Request builder helpers
// ([UserMessage], [AssistantMessage], [WithModel], etc.) compose
// [CreateMessageRequest] values ergonomically.
//
// Example:
//
//	client := sampling.ClientFromContext(ctx)
//	resp, err := client.CreateMessage(ctx, sampling.NewRequest(
//	    sampling.UserMessage("What is the capital of France?"),
//	    sampling.WithModel("claude-3-5-sonnet"),
//	))
package sampling
