//go:build !official_sdk

package roots_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/roots"
)

// staticRootsClient is a minimal RootsClient for examples.
type staticRootsClient struct {
	items []roots.Root
}

func (c *staticRootsClient) ListRoots(_ context.Context) ([]roots.Root, error) {
	return c.items, nil
}

func ExampleWithRootsClient() {
	client := &staticRootsClient{
		items: []roots.Root{
			{URI: "file:///workspace", Name: "workspace"},
		},
	}

	ctx := roots.WithRootsClient(context.Background(), client)
	r := roots.ClientFromContext(ctx)
	fmt.Println(r != nil)
	// Output:
	// true
}

func ExampleListRoots() {
	client := &staticRootsClient{
		items: []roots.Root{
			{URI: "file:///home/user/project", Name: "project"},
		},
	}

	ctx := roots.WithRootsClient(context.Background(), client)
	list, err := roots.ListRoots(ctx)
	fmt.Println(err)
	fmt.Println(len(list))
	fmt.Println(list[0].Name)
	// Output:
	// <nil>
	// 1
	// project
}

func ExampleNewCachedClient() {
	inner := &staticRootsClient{
		items: []roots.Root{
			{URI: "file:///data", Name: "data"},
		},
	}

	cached := roots.NewCachedClient(inner)
	list, err := cached.ListRoots(context.Background())
	fmt.Println(err)
	fmt.Println(len(list))

	cached.Invalidate()
	list2, _ := cached.ListRoots(context.Background())
	fmt.Println(len(list2))
	// Output:
	// <nil>
	// 1
	// 1
}
