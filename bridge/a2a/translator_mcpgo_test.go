//go:build !official_sdk

// translator_mcpgo_test.go covers a genuinely mcp-go-specific edge case:
// contentToPart's fallback behavior when ImageContent.Data isn't valid
// base64. mcp-go's ImageContent.Data is a base64-encoded string, so
// malformed data is a real possibility contentToPart has to handle; the
// official SDK's ImageContent.Data is already raw []byte, so "invalid
// base64 image data" isn't an expressible concept there at all — there is
// no official_sdk counterpart to this test, not because of a missing
// compat feature but because the scenario itself doesn't exist on that
// build.
package a2a

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestContentToPart_InvalidBase64Image(t *testing.T) {
	t.Parallel()

	content := mcp.ImageContent{
		Type:     "image",
		Data:     "not-valid-base64!!!",
		MIMEType: "image/png",
	}

	part := contentToPart(content)
	if part == nil {
		t.Fatal("expected non-nil part even for invalid base64")
	}
	// registry.ExtractImageContent treats an undecodable Data as "not
	// extractable as image content" (see its doc comment), so contentToPart
	// falls through to the generic JSON-serialization default case instead
	// of the old text-with-raw-base64-string fallback.
	data := part.Data()
	if data == nil {
		t.Fatalf("expected a DataPart fallback for undecodable image content, got %+v", part)
	}
}
