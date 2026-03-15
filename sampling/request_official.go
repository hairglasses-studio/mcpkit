//go:build official_sdk

package sampling

import "github.com/hairglasses-studio/mcpkit/registry"

// WithMaxTokens sets the max tokens for the completion.
// In the official SDK, MaxTokens is int64.
func WithMaxTokens(n int) RequestOption {
	return func(p *CreateMessageParams) { p.MaxTokens = int64(n) }
}

// TextMessage creates a SamplingMessage with plain text content.
// In the official SDK, Messages is []*SamplingMessage, so we return a pointer-compatible value.
func TextMessage(role, text string) SamplingMessage {
	return SamplingMessage{
		Role:    registry.Role(role),
		Content: registry.MakeTextContent(text),
	}
}

// CompletionRequest builds a CreateMessageRequest from messages and options.
// The default MaxTokens is 1024 if not overridden by a WithMaxTokens option.
//
// In the official SDK, CreateMessageRequest is ClientRequest[*CreateMessageParams],
// and Messages is []*SamplingMessage.
func CompletionRequest(messages []SamplingMessage, opts ...RequestOption) CreateMessageRequest {
	ptrs := make([]*SamplingMessage, len(messages))
	for i := range messages {
		m := messages[i]
		ptrs[i] = &m
	}
	params := CreateMessageParams{
		Messages:  ptrs,
		MaxTokens: 1024,
	}
	for _, opt := range opts {
		opt(&params)
	}
	return CreateMessageRequest{
		Params: &params,
	}
}
