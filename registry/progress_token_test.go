package registry

import "testing"

// TestProgressTokenFromRequest_NilMeta is portable: on both tags, a request
// with no _meta at all has no progress token.
func TestProgressTokenFromRequest_NilMeta(t *testing.T) {
	req := CallToolRequest{}
	if got := ProgressTokenFromRequest(req); got != nil {
		t.Errorf("ProgressTokenFromRequest(nil Meta) = %v, want nil", got)
	}
}
