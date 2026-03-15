// Package client provides client utilities for MCP tool modules.
package client_test

import (
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/client"
)

func ExampleStandard() {
	c := client.Standard()
	fmt.Println(c != nil)
	fmt.Println(c.Timeout)
	// Output:
	// true
	// 30s
}

func ExampleFast() {
	c := client.Fast()
	fmt.Println(c != nil)
	fmt.Println(c.Timeout)
	// Output:
	// true
	// 5s
}

func ExampleWithTimeout() {
	c := client.WithTimeout(10 * time.Second)
	fmt.Println(c != nil)
	fmt.Println(c.Timeout)
	// Output:
	// true
	// 10s
}

func ExampleLazyClient() {
	calls := 0
	get := client.LazyClient(func() (string, error) {
		calls++
		return "connected", nil
	})

	v1, _ := get()
	v2, _ := get()

	fmt.Println(v1)
	fmt.Println(v2)
	fmt.Println(calls) // constructor called only once
	// Output:
	// connected
	// connected
	// 1
}
