//go:build !official_sdk

// invoke.go provides a dual-implemented way to invoke a ResourceHandlerFunc
// directly in a test (bypassing the registry/server) and read back a single
// text result — the piece consumer test code needs but cannot write
// tag-free on its own: ResourceHandlerFunc's request AND result shapes
// differ per SDK (mcp-go: func(ctx, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error);
// official: func(ctx, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)).
// See invoke_official.go for the official_sdk counterpart.
package resources

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// CallHandlerText invokes h directly (no registry/server involved) with a
// request for uri, and extracts the first content entry's MIME type and
// text. Returns an error if h errors, returns no contents, or its first
// content entry isn't text — covering exactly what real consumer test code
// checks (non-empty contents, first entry parsed as text) without each call
// site needing to construct an SDK-specific request or type-assert the
// SDK-specific result shape itself.
func CallHandlerText(ctx context.Context, h ResourceHandlerFunc, uri string) (mimeType, text string, err error) {
	contents, err := h(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		return "", "", err
	}
	if len(contents) == 0 {
		return "", "", fmt.Errorf("resources: handler returned no contents for %q", uri)
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		return "", "", fmt.Errorf("resources: first content for %q is %T, want TextResourceContents", uri, contents[0])
	}
	return tc.MIMEType, tc.Text, nil
}
