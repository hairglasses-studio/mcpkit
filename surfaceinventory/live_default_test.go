//go:build !official_sdk

package surfaceinventory

import (
	"context"
	"strings"
	"testing"
)

// TestConnectAndListNames_DefaultStub confirms the default (mcp-go) build
// fails closed with a clear, actionable error rather than a silent empty
// diff or a panic — surface_inventory_live is official_sdk-only because
// mcp-go's client has no equivalent of go-sdk's auto-paginating list
// iterators.
func TestConnectAndListNames_DefaultStub(t *testing.T) {
	t.Parallel()

	names, err := connectAndListNames(context.Background(), []string{"irrelevant-binary"}, KindMCPTool)
	if err == nil {
		t.Fatal("expected an error under the default build tag, got nil")
	}
	if names != nil {
		t.Errorf("expected nil names, got %v", names)
	}
	if !strings.Contains(err.Error(), "official_sdk") {
		t.Errorf("error should mention official_sdk, got: %v", err)
	}
}
