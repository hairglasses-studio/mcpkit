//go:build !official_sdk

// handler_adapters.go provides SDK-neutral constructors for the two common
// resource handler shapes (P52.6/P52.7 canary unblocker: consumer handler
// code — e.g. secretstudios-mcp's internal/surface/resources.go — cannot
// currently be written once for both build tags because ResourceHandlerFunc
// itself differs per SDK: mcp-go's is
// func(ctx, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error); the
// official SDK's is func(ctx, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error).
// TextResourceHandler and JSONResourceHandler adapt a plain, SDK-free inner
// function into this build's real ResourceHandlerFunc, so a consumer writes
// the inner function once and gets a working handler under both tags. See
// handler_adapters_official.go for the official_sdk counterpart.
package resources

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// TextResourceHandler adapts a (ctx, uri) -> (mimeType, text, err) function
// into a ResourceHandlerFunc that returns exactly one text resource content
// entry. Covers the common case of a resource that always serves a single
// fixed-mimeType text body (e.g. static markdown documentation).
func TextResourceHandler(fn func(ctx context.Context, uri string) (mimeType, text string, err error)) ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		mimeType, text, err := fn(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: mimeType,
				Text:     text,
			},
		}, nil
	}
}

// JSONResourceHandler adapts a (ctx, uri) -> (any, error) function into a
// ResourceHandlerFunc that JSON-marshals the returned value and serves it as
// a single application/json text resource content entry.
func JSONResourceHandler(fn func(ctx context.Context, uri string) (any, error)) ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		data, err := fn(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(b),
			},
		}, nil
	}
}
