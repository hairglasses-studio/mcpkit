// Package client provides shared HTTP client utilities for MCP tool modules.
//
// It exposes three pre-tuned singleton [http.Client] instances — [Fast] (5s
// timeout for LAN/local calls), [Standard] (30s for cloud APIs), and [Slow]
// (2m for uploads and long-running requests) — all backed by a shared
// transport with connection pooling and keep-alive. [LazyClient] is a generic
// helper for thread-safe, once-initialized API clients, ensuring expensive
// construction happens at most once per process.
//
// Example:
//
//	var getGitHubClient = client.LazyClient(func() (*github.Client, error) {
//	    return github.NewClient(client.Standard()), nil
//	})
package client
