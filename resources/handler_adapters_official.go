//go:build official_sdk

// handler_adapters_official.go is the official_sdk counterpart to
// handler_adapters.go — same TextResourceHandler/JSONResourceHandler
// signatures, adapted to the official SDK's pointer-based
// *mcp.ReadResourceRequest / *mcp.ReadResourceResult / []*mcp.ResourceContents
// shapes instead of mcp-go's value-based ones.
package resources

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TextResourceHandler adapts a (ctx, uri) -> (mimeType, text, err) function
// into a ResourceHandlerFunc that returns exactly one text resource content
// entry. Covers the common case of a resource that always serves a single
// fixed-mimeType text body (e.g. static markdown documentation).
func TextResourceHandler(fn func(ctx context.Context, uri string) (mimeType, text string, err error)) ResourceHandlerFunc {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		mimeType, text, err := fn(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: mimeType,
					Text:     text,
				},
			},
		}, nil
	}
}

// JSONResourceHandler adapts a (ctx, uri) -> (any, error) function into a
// ResourceHandlerFunc that JSON-marshals the returned value and serves it as
// a single application/json text resource content entry.
func JSONResourceHandler(fn func(ctx context.Context, uri string) (any, error)) ResourceHandlerFunc {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := fn(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(b),
				},
			},
		}, nil
	}
}
