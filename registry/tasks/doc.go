// Package tasks provides async task lifecycle management for long-running MCP
// tool operations.
//
// The Manager interface tracks task state (running, completed, failed, cancelled)
// with thread-safe in-memory storage and automatic TTL-based expiration. Tasks
// integrate with the MCP progress notification protocol, allowing clients to
// poll status and receive completion results. A middleware wrapper automatically
// converts long-running tool handlers into async tasks with progress reporting.
package tasks
