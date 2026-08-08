//go:build official_sdk

// handler_adapters_official.go is the official_sdk counterpart to
// handler_adapters.go — same TextPromptHandler signature, adapted to the
// official SDK's pointer-based *mcp.GetPromptRequest /
// []*mcp.PromptMessage / *mcp.TextContent shapes instead of mcp-go's
// value-based ones. go-sdk has no NewPromptMessage/NewTextContent
// convenience constructors (unlike mcp-go), so the message is built
// directly from struct literals.
package prompts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TextPromptHandler adapts a (ctx, args) -> (description, text, err)
// function into a PromptHandlerFunc whose result is exactly one user-role
// text message.
func TextPromptHandler(fn func(ctx context.Context, args map[string]string) (description, text string, err error)) PromptHandlerFunc {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		description, text, err := fn(ctx, req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{
			Description: description,
			Messages: []*mcp.PromptMessage{
				{Role: registry.RoleUser, Content: &mcp.TextContent{Text: text}},
			},
		}, nil
	}
}
