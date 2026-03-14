// Package client provides client utilities for MCP tool modules.
package client

import "sync"

// LazyClient returns a thread-safe lazy-initialized client getter.
// The constructor is called at most once; subsequent calls return the cached result.
//
// Usage:
//
//	var getClient = client.LazyClient(newMyAPIClient)
func LazyClient[T any](constructor func() (T, error)) func() (T, error) {
	var client T
	var once sync.Once
	var initErr error
	return func() (T, error) {
		once.Do(func() { client, initErr = constructor() })
		return client, initErr
	}
}
