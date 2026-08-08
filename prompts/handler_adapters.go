//go:build !official_sdk

// handler_adapters.go provides an SDK-neutral constructor for the common
// prompt handler shape (P52.6/P52.7 canary unblocker, same rationale as
// resources/handler_adapters.go): PromptHandlerFunc itself differs per SDK
// (mcp-go: func(ctx, mcp.GetPromptRequest) (*mcp.GetPromptResult, error);
// official SDK: func(ctx, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error),
// with []mcp.PromptMessage vs []*mcp.PromptMessage inside the result).
// TextPromptHandler adapts a plain, SDK-free inner function into this
// build's real PromptHandlerFunc. Covers every prompt handler in
// secretstudios-mcp's internal/surface/prompts.go, which returns exactly one
// user-role text message built from a (description, text) pair.
package prompts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// TextPromptHandler adapts a (ctx, args) -> (description, text, err)
// function into a PromptHandlerFunc whose result is exactly one user-role
// text message.
func TextPromptHandler(fn func(ctx context.Context, args map[string]string) (description, text string, err error)) PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		description, text, err := fn(ctx, req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{
			Description: description,
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
			},
		}, nil
	}
}
