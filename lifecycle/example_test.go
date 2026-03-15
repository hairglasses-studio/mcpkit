//go:build !official_sdk

package lifecycle_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/lifecycle"
)

func ExampleNew() {
	mgr := lifecycle.New(lifecycle.Config{
		OnDraining: func() {},
	})

	mgr.OnShutdown(func(ctx context.Context) error {
		return nil
	})

	fmt.Println(mgr.Status())
	// Output:
	// created
}
