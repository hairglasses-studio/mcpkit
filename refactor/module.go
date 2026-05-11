package refactor

import (
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Module implements registry.ToolModule for refactoring and conversion pipeline tools.
type Module struct{}

// NewModule creates a new refactor Module.
func NewModule() *Module {
	return &Module{}
}

// Name returns the module name.
func (m *Module) Name() string { return "refactor" }

// Description returns the module description.
func (m *Module) Description() string {
	return "Automated pipeline tools to repeatedly aid in code refactoring (e.g., bash/python to Go/Rust)"
}

// Tools returns all refactor tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		discoverScriptsToolDef(),
		analyzeScriptToolDef(),
		planConversionToolDef(),
	}
}
