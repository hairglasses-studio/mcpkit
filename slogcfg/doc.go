// Package slogcfg provides a shared structured logging configuration for MCP servers.
//
// It replaces the identical 3-4 line slog.SetDefault boilerplate found across
// dotfiles-mcp, process-mcp, systemd-mcp, tmux-mcp, and ralphglasses with a
// single Init(Config) call.
//
// Usage:
//
//	slogcfg.Init(slogcfg.Config{
//	    ServiceName: "my-mcp-server",
//	})
package slogcfg
