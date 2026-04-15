//go:build !official_sdk

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "rdloop is disabled to conserve Anthropic budget for active coding sessions")
	os.Exit(1)
}
