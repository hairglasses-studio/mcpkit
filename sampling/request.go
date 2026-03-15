//go:build !official_sdk

package sampling

import "github.com/hairglasses-studio/mcpkit/registry"

// WithMaxTokens sets the max tokens for the completion.
func WithMaxTokens(n int) RequestOption {
	return func(p *CreateMessageParams) { p.MaxTokens = n }
}

// TextMessage creates a SamplingMessage with plain text content.
func TextMessage(role, text string) SamplingMessage {
	return SamplingMessage{
		Role:    registry.Role(role),
		Content: registry.MakeTextContent(text),
	}
}

// CompletionRequest builds a CreateMessageRequest from messages and options.
// The default MaxTokens is 1024 if not overridden by a WithMaxTokens option.
func CompletionRequest(messages []SamplingMessage, opts ...RequestOption) CreateMessageRequest {
	params := CreateMessageParams{
		Messages:  messages,
		MaxTokens: 1024,
	}
	for _, opt := range opts {
		opt(&params)
	}
	return CreateMessageRequest{
		CreateMessageParams: params,
	}
}
