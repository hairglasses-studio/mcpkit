//go:build !official_sdk

package roots

import (
	"context"
	"testing"
)

func TestServerRootsClient_NoSession(t *testing.T) {
	t.Parallel()
	client := &ServerRootsClient{}
	_, err := client.ListRoots(context.Background())
	if err != ErrRootsUnavailable {
		t.Errorf("expected ErrRootsUnavailable, got %v", err)
	}
}
