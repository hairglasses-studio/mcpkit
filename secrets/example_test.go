//go:build !official_sdk

package secrets_test

import (
	"context"
	"fmt"
	"os"

	"github.com/hairglasses-studio/mcpkit/secrets"
	"github.com/hairglasses-studio/mcpkit/secrets/providers"
)

func ExampleNewManager() {
	env := providers.NewEnvProvider()

	mgr := secrets.NewManager(
		secrets.WithProviders(env),
	)

	fmt.Println(len(mgr.Providers()))
	// Output:
	// 1
}

func ExampleManager_GetWithFallback() {
	mgr := secrets.NewManager()

	val := mgr.GetWithFallback(context.Background(), "MCPKIT_NONEXISTENT_KEY_XYZ", "default-value")
	fmt.Println(val)
	// Output:
	// default-value
}

func ExampleNewEnvProvider() {
	// Set a test env var for demonstration.
	os.Setenv("MCPKIT_EXAMPLE_KEY", "my-secret")
	defer os.Unsetenv("MCPKIT_EXAMPLE_KEY")

	p := providers.NewEnvProvider()
	secret, err := p.Get(context.Background(), "MCPKIT_EXAMPLE_KEY")
	fmt.Println(err)
	fmt.Println(secret.Value)
	// Output:
	// <nil>
	// my-secret
}
