//go:build !official_sdk

// Package prefetch provides context pre-loading middleware for mcpkit tool invocations.
//
// Factor 13 from 12-Factor Agents: "Pre-Fetch All Context You Might Need" -- if
// you know the agent will need certain data, fetch it upfront instead of making
// the LLM request it via tool calls. This saves tokens and latency.
//
// Providers register named data fetchers that run before tool execution. Each
// provider declares which tools it should pre-fetch for via ShouldPrefetch. Data
// is cached per TTL and injected into the tool's context via context.WithValue,
// allowing handlers to retrieve it with PrefetchFromContext.
//
// Usage:
//
//	mw := prefetch.New(
//	    prefetch.WithProvider("git_status", fetchGitStatus, matchGitTools),
//	    prefetch.WithCacheTTL(10 * time.Minute),
//	    prefetch.WithMaxConcurrent(8),
//	)
package prefetch
