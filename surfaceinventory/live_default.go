//go:build !official_sdk

package surfaceinventory

import (
	"context"
	"fmt"
)

// connectAndListNames is a stub under the default (mcp-go) build tag.
// mcp-go's client does not expose an equivalent of the official go-sdk's
// ClientSession.Tools/Resources/Prompts auto-paginating iterators
// (iter.Seq2[*T, error]) that connectAndListNames in live_official.go
// relies on, so surface_inventory_live is only available when built with
// -tags official_sdk. Callers get a clear, actionable error instead of a
// silently-empty or incorrect diff.
func connectAndListNames(_ context.Context, _ []string, _ string) ([]string, error) {
	return nil, fmt.Errorf("surface_inventory_live requires the official_sdk build tag: " +
		"rebuild with `go build -tags official_sdk` (or `go run -tags official_sdk`) — " +
		"mcp-go's client does not expose the go-sdk's auto-paginating list iterators this tool depends on")
}
