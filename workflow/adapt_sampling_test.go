//go:build !official_sdk

package workflow

import (
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

func newSamplingTextResult(text, model string) *sampling.CreateMessageResult {
	return &sampling.CreateMessageResult{
		SamplingMessage: sampling.SamplingMessage{
			Role:    registry.RoleAssistant,
			Content: registry.MakeTextContent(text),
		},
		Model: model,
	}
}

func newSamplingNonTextResult() *sampling.CreateMessageResult {
	return &sampling.CreateMessageResult{
		SamplingMessage: sampling.SamplingMessage{
			Role:    registry.RoleAssistant,
			Content: "not a TextContent",
		},
	}
}
