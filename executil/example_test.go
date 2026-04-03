//go:build !official_sdk

package executil_test

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/executil"
)

func ExampleOutput() {
	out, err := executil.Output(context.Background(), "echo", "hello world")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(out)
	// Output:
	// hello world
}

func ExampleOutputTimeout() {
	out, err := executil.OutputTimeout(5*time.Second, "echo", "fast")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(out)
	// Output:
	// fast
}
