//go:build !official_sdk

package registry

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestProgressTokenFromRequest_Present confirms real extraction on mcp-go —
// the official_sdk build always returns nil (see compat_official.go's
// ProgressTokenFromRequest doc comment: go-sdk v1.7.0 has no per-session
// progress notifications yet), so this behavior is deliberately not
// asserted in a neutral test file.
func TestProgressTokenFromRequest_Present(t *testing.T) {
	req := CallToolRequest{
		Params: mcp.CallToolParams{
			Meta: &mcp.Meta{ProgressToken: mcp.ProgressToken("tok-123")},
		},
	}
	got := ProgressTokenFromRequest(req)
	token, ok := got.(mcp.ProgressToken)
	if !ok || token != "tok-123" {
		t.Errorf("ProgressTokenFromRequest = %v (%T), want mcp.ProgressToken(\"tok-123\")", got, got)
	}
}
