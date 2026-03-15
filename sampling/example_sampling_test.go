//go:build !official_sdk

package sampling_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/sampling"
)

func ExampleWithSamplingClient() {
	// Attach a nil client (placeholder) to verify context helpers work.
	ctx := sampling.WithSamplingClient(context.Background(), nil)
	client := sampling.ClientFromContext(ctx)
	fmt.Println(client == nil)
	// Output:
	// true
}

func ExampleMiddleware() {
	mw := sampling.Middleware(nil)
	fmt.Println(mw != nil)
	// Output:
	// true
}
