//go:build official_sdk

package workflow

import (
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

func newSamplingTextResult(text, model string) *sampling.CreateMessageResult {
	result := &sampling.CreateMessageResult{
		Model: model,
	}
	result.Role = registry.RoleAssistant
	result.Content = registry.MakeTextContent(text)
	return result
}

func newSamplingNonTextResult() *sampling.CreateMessageResult {
	result := &sampling.CreateMessageResult{}
	result.Role = registry.RoleAssistant
	return result
}
