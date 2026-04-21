package frontdoor

import (
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ModuleName is the registry module name used by frontdoor.
const ModuleName = "frontdoor"

// Module implements registry.ToolModule with the four discovery tools.
// Construct with New and register on a ToolRegistry via RegisterModule.
type Module struct {
	reg     *registry.ToolRegistry
	prefix  string
	checker *health.Checker
}

// Option configures a Module.
type Option func(*Module)

// WithPrefix prepends a prefix to every tool name, e.g. WithPrefix("myapp_")
// exposes the module as myapp_tool_catalog, myapp_tool_search, and so on.
// An empty prefix (the default) keeps the bare tool names.
func WithPrefix(prefix string) Option {
	return func(m *Module) { m.prefix = prefix }
}

// WithHealthChecker wires a health.Checker so server_health reports the
// checker's lifecycle status and uptime. When not set, server_health returns
// a static "ok" status plus tool inventory counts from the registry.
func WithHealthChecker(c *health.Checker) Option {
	return func(m *Module) { m.checker = c }
}

// New constructs a front-door module bound to reg.
func New(reg *registry.ToolRegistry, opts ...Option) *Module {
	m := &Module{reg: reg}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Name returns the module name.
func (m *Module) Name() string { return ModuleName }

// Description returns a short module description.
func (m *Module) Description() string {
	return "Discovery-first front door: tool_catalog, tool_search, tool_schema, server_health"
}

// Tools returns the four discovery tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		m.catalogTool(),
		m.searchTool(),
		m.schemaTool(),
		m.healthTool(),
	}
}

func (m *Module) toolName(suffix string) string {
	if m.prefix == "" {
		return suffix
	}
	return m.prefix + suffix
}
