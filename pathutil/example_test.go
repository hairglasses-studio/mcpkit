//go:build !official_sdk

package pathutil_test

import (
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/pathutil"
)

func ExampleExpandHome() {
	// ExpandHome replaces ~ with the user's home directory.
	path := pathutil.ExpandHome("~/projects")
	fmt.Println(strings.HasSuffix(path, "/projects"))
	// Output:
	// true
}

func ExampleIsSubPath() {
	fmt.Println(pathutil.IsSubPath("/home/user", "/home/user/docs"))
	fmt.Println(pathutil.IsSubPath("/home/user", "/etc/passwd"))
	// Output:
	// true
	// false
}

func ExampleResolveEnv() {
	// Returns fallback when env var is unset.
	val := pathutil.ResolveEnv("UNLIKELY_TO_EXIST_PATHUTIL_EXAMPLE", "/default/path")
	fmt.Println(val)
	// Output:
	// /default/path
}
