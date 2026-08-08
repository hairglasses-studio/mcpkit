//go:build official_sdk

// invoke_official.go is the official_sdk counterpart to invoke.go — same
// CallHandlerText signature, adapted to the official SDK's pointer-based
// *mcp.ReadResourceRequest / *mcp.ReadResourceResult / []*mcp.ResourceContents
// shapes instead of mcp-go's value-based ones.
package resources

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CallHandlerText invokes h directly (no registry/server involved) with a
// request for uri, and extracts the first content entry's MIME type and
// text. Returns an error if h errors, returns no contents, or its first
// content entry is nil.
func CallHandlerText(ctx context.Context, h ResourceHandlerFunc, uri string) (mimeType, text string, err error) {
	result, err := h(ctx, &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		return "", "", err
	}
	if result == nil || len(result.Contents) == 0 {
		return "", "", fmt.Errorf("resources: handler returned no contents for %q", uri)
	}
	c := result.Contents[0]
	if c == nil {
		return "", "", fmt.Errorf("resources: first content for %q is nil", uri)
	}
	return c.MIMEType, c.Text, nil
}
