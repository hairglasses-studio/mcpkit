//go:build !official_sdk

package mcptest

import "github.com/hairglasses-studio/mcpkit/registry"

func setSamplingMaxTokens(req *registry.CreateMessageRequest, maxTokens int) {
	req.MaxTokens = maxTokens
}

func samplingMaxTokens(req registry.CreateMessageRequest) int {
	return req.MaxTokens
}
