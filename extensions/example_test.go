// Package extensions provides MCP Extensions negotiation and capability handshake.
package extensions_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/extensions"
)

func ExampleNewExtensionRegistry() {
	reg := extensions.NewExtensionRegistry()

	err := reg.Register(extensions.Extension{
		Name:    "mcpkit:health",
		Version: "1.0.0",
	})

	fmt.Println(err)
	fmt.Println(len(reg.Available()))
	// Output:
	// <nil>
	// 1
}

func ExampleExtensionRegistry_Negotiate() {
	reg := extensions.NewExtensionRegistry()
	_ = reg.Register(extensions.Extension{
		Name:    "mcpkit:tracing",
		Version: "1.0.0",
	})

	results := reg.Negotiate([]string{"mcpkit:tracing"})

	fmt.Println(len(results))
	fmt.Println(results[0].Accepted)
	fmt.Println(reg.IsActive("mcpkit:tracing"))
	// Output:
	// 1
	// true
	// true
}

func ExampleExtensionRegistry_IsActive() {
	reg := extensions.NewExtensionRegistry()
	_ = reg.Register(extensions.Extension{
		Name:    "mcpkit:finops",
		Version: "1.0.0",
	})

	fmt.Println(reg.IsActive("mcpkit:finops")) // not yet negotiated

	reg.Negotiate([]string{"mcpkit:finops"})

	fmt.Println(reg.IsActive("mcpkit:finops")) // now active
	// Output:
	// false
	// true
}
