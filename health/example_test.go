//go:build !official_sdk

package health_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/health"
)

func ExampleNewChecker() {
	checker := health.NewChecker(
		health.WithToolCount(func() int { return 5 }),
	)

	status := checker.Check()
	fmt.Println(status.Status)
	fmt.Println(status.ToolCount)
	// Output:
	// ok
	// 5
}

func ExampleChecker_IsReady() {
	checker := health.NewChecker()

	fmt.Println(checker.IsReady())

	checker.SetStatus("draining")
	fmt.Println(checker.IsReady())
	// Output:
	// true
	// false
}
