//go:build !official_sdk

package bootstrap

import (
	"log/slog"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// Option configures the bootstrap server.
type Option func(*serveOptions)

type serveOptions struct {
	modules          []registry.ToolModule
	middleware       []registry.Middleware
	registryConfig   registry.Config
	auditPath        string
	safetyTiers      bool
	logHandler       slog.Handler
	logLevel         slog.Level
	resourceRegistry *resources.ResourceRegistry
	promptRegistry   *prompts.PromptRegistry
	serverOpts       []server.ServerOption
}

func defaultServeOptions() serveOptions {
	return serveOptions{
		logLevel: slog.LevelInfo,
	}
}

// WithModule registers a tool module.
func WithModule(m registry.ToolModule) Option {
	return func(o *serveOptions) {
		o.modules = append(o.modules, m)
	}
}

// WithModules registers multiple tool modules.
func WithModules(mods ...registry.ToolModule) Option {
	return func(o *serveOptions) {
		o.modules = append(o.modules, mods...)
	}
}

// WithMiddleware adds custom middleware to the registry.
func WithMiddleware(mw registry.Middleware) Option {
	return func(o *serveOptions) {
		o.middleware = append(o.middleware, mw)
	}
}

// WithRegistryConfig sets the full registry config. Middleware added via
// WithMiddleware or WithAudit/WithSafetyTiers is appended after any
// middleware already present in this config.
func WithRegistryConfig(cfg registry.Config) Option {
	return func(o *serveOptions) {
		o.registryConfig = cfg
	}
}

// WithAudit enables audit logging to the given file path.
// Pass an empty string to use the default XDG_STATE_HOME path.
func WithAudit(path string) Option {
	return func(o *serveOptions) {
		o.auditPath = path
	}
}

// WithSafetyTiers enables safety tier classification middleware.
func WithSafetyTiers() Option {
	return func(o *serveOptions) {
		o.safetyTiers = true
	}
}

// WithLogHandler sets a custom slog handler.
func WithLogHandler(h slog.Handler) Option {
	return func(o *serveOptions) {
		o.logHandler = h
	}
}

// WithLogLevel sets the minimum log level.
func WithLogLevel(level slog.Level) Option {
	return func(o *serveOptions) {
		o.logLevel = level
	}
}

// WithResources sets the resource registry. The server will automatically
// be configured with resource capabilities.
func WithResources(r *resources.ResourceRegistry) Option {
	return func(o *serveOptions) {
		o.resourceRegistry = r
	}
}

// WithPrompts sets the prompt registry. The server will automatically
// be configured with prompt capabilities.
func WithPrompts(p *prompts.PromptRegistry) Option {
	return func(o *serveOptions) {
		o.promptRegistry = p
	}
}

// WithServerOption adds a raw mcp-go server option for advanced configuration.
func WithServerOption(opt server.ServerOption) Option {
	return func(o *serveOptions) {
		o.serverOpts = append(o.serverOpts, opt)
	}
}
