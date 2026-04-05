//go:build !official_sdk

package bootstrap

import (
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ServerConfig holds the server identity for bootstrap.Serve.
// This is separate from Config (used by GenerateReport) to avoid
// overloading a single type.
type ServerConfig struct {
	Name    string
	Version string
}

// Serve creates and runs an MCP server with the given config and options.
// It sets up structured logging, creates a tool registry, applies middleware,
// registers modules, and serves via auto-detected transport (Unix socket if
// MCP_SOCKET_PATH is set, otherwise stdio).
func Serve(cfg ServerConfig, opts ...Option) error {
	o := defaultServeOptions()
	for _, opt := range opts {
		opt(&o)
	}

	// Setup structured logging to stderr.
	if o.logHandler == nil {
		o.logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: o.logLevel})
	}
	slog.SetDefault(slog.New(o.logHandler))

	// Build the middleware chain: audit and safety tiers first, then user middleware.
	var mw []registry.Middleware
	if o.auditPath != "" {
		mw = append(mw, registry.AuditMiddleware(o.auditPath))
	}
	if o.safetyTiers {
		mw = append(mw, registry.SafetyTierMiddleware())
	}
	mw = append(mw, o.middleware...)

	// Merge middleware into the registry config.
	regCfg := o.registryConfig
	regCfg.Middleware = append(regCfg.Middleware, mw...)

	// Create registry and register tool modules.
	reg := registry.NewToolRegistry(regCfg)
	for _, mod := range o.modules {
		reg.RegisterModule(mod)
	}

	// Build server options: always enable tool capabilities.
	serverOpts := []server.ServerOption{
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	}
	if o.resourceRegistry != nil {
		serverOpts = append(serverOpts, server.WithResourceCapabilities(false, true))
	}
	if o.promptRegistry != nil {
		serverOpts = append(serverOpts, server.WithPromptCapabilities(true))
	}
	serverOpts = append(serverOpts, o.serverOpts...)

	// Create and wire the MCP server.
	srv := registry.NewMCPServer(cfg.Name, cfg.Version, serverOpts...)
	reg.RegisterWithServer(srv)

	if o.resourceRegistry != nil {
		o.resourceRegistry.RegisterWithServer(srv)
	}
	if o.promptRegistry != nil {
		o.promptRegistry.RegisterWithServer(srv)
	}

	return registry.ServeAuto(srv)
}
