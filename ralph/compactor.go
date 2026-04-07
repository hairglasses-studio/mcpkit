package ralph

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// ErrorCompactor (Factor 9) distills verbose failure outputs into concise summaries.
type ErrorCompactor struct {
	Sampler   sampling.SamplingClient
	Threshold int // Max length of error string before compaction triggers (default 512)
}

// Compact summarizes a verbose error message into a single-line hint.
func (c *ErrorCompactor) Compact(ctx context.Context, errStr string) string {
	threshold := c.Threshold
	if threshold <= 0 {
		threshold = 512
	}

	if len(errStr) <= threshold {
		return errStr
	}

	if c.Sampler == nil {
		return errStr[:threshold] + "..."
	}

	prompt := fmt.Sprintf(`The following tool execution failed with a verbose error. 
Distill the most critical failure reason into a single concise sentence for the next iteration.

ERROR:
%s`, errStr)

	resp, err := c.Sampler.CreateMessage(ctx, sampling.CreateMessageRequest{
		CreateMessageParams: sampling.CreateMessageParams{
			Messages: []sampling.SamplingMessage{
				sampling.TextMessage("user", prompt),
			},
			MaxTokens: 64,
		},
	})
	if err != nil {
		return errStr[:threshold] + "..."
	}

	if content, ok := resp.Content.(registry.Content); ok {
		if text, ok := registry.ExtractTextContent(content); ok {
			return strings.TrimSpace(text)
		}
	}

	return errStr[:threshold] + "..."
}
