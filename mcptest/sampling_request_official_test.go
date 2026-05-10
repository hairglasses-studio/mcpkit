//go:build official_sdk

package mcptest

import "github.com/hairglasses-studio/mcpkit/registry"

func setSamplingMaxTokens(req *registry.CreateMessageRequest, maxTokens int) {
	if req.Params == nil {
		req.Params = &registry.CreateMessageParams{}
	}
	req.Params.MaxTokens = int64(maxTokens)
}

func samplingMaxTokens(req registry.CreateMessageRequest) int {
	if req.Params == nil {
		return 0
	}
	return int(req.Params.MaxTokens)
}
