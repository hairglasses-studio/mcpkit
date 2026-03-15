//go:build !official_sdk

package gateway

import (
	"fmt"
)

// ExampleNewGateway demonstrates creating a gateway and verifying that the
// accompanying DynamicRegistry is returned ready to use.
func ExampleNewGateway() {
	gw, reg := NewGateway()
	defer gw.Close()

	// No upstreams yet — the registry should report zero tools.
	tools := reg.ListTools()
	fmt.Println("tool count:", len(tools))

	// No upstreams registered yet.
	upstreams := gw.ListUpstreams()
	fmt.Println("upstream count:", len(upstreams))
	// Output:
	// tool count: 0
	// upstream count: 0
}
