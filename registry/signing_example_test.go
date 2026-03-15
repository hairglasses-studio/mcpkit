//go:build !official_sdk

package registry_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleSignTool() {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)

	td := registry.ToolDefinition{
		Tool:    registry.Tool{Name: "search", Description: "Search things"},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) { return nil, nil },
	}

	sig := registry.SignTool(td, priv, "builder")
	err := registry.VerifyToolSignature(td, sig, pub)
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleSignatureStore_SignAll() {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})

	store := registry.NewSignatureStore()
	store.SignAll(reg, priv, "ci")

	td, _ := reg.GetTool("echo_text")
	err := store.Verify(td)
	fmt.Println(err)
	// Output:
	// <nil>
}
