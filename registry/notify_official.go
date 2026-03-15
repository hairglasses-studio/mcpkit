//go:build official_sdk

package registry

// WireToolListChanged is a no-op for the official SDK variant.
// The official SDK does not yet expose broadcast notification APIs.
func WireToolListChanged(d *DynamicRegistry, s *MCPServer) {
	// no-op: official SDK lacks notification broadcast support
}
