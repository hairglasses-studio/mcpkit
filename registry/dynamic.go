package registry

import (
	"sort"
	"sync"
)

// ChangeNotifier is called when the tool list changes at runtime.
// Implement this to send tools/list_changed notifications to MCP clients.
type ChangeNotifier func()

// DynamicRegistry extends ToolRegistry with runtime tool registration/removal
// and change notification support for the MCP tools/list_changed protocol.
type DynamicRegistry struct {
	*ToolRegistry
	notifierMu sync.RWMutex
	notifiers  []ChangeNotifier
}

// NewDynamicRegistry creates a registry that supports runtime tool changes.
func NewDynamicRegistry(config ...Config) *DynamicRegistry {
	return &DynamicRegistry{
		ToolRegistry: NewToolRegistry(config...),
	}
}

// OnChange registers a callback that fires when tools are added or removed.
func (d *DynamicRegistry) OnChange(fn ChangeNotifier) {
	d.notifierMu.Lock()
	defer d.notifierMu.Unlock()
	d.notifiers = append(d.notifiers, fn)
}

func (d *DynamicRegistry) notify() {
	d.notifierMu.RLock()
	defer d.notifierMu.RUnlock()
	for _, fn := range d.notifiers {
		fn()
	}
}

// AddTool registers a single tool at runtime and notifies listeners.
func (d *DynamicRegistry) AddTool(td ToolDefinition) {
	d.mu.Lock()
	if !td.IsWrite {
		td.IsWrite = InferIsWrite(td.Tool.Name)
	}
	if td.RuntimeGroup == "" && d.config.RuntimeGroupMapper != nil {
		td.RuntimeGroup = d.config.RuntimeGroupMapper(td.Category)
	}
	d.tools[td.Tool.Name] = td
	d.mu.Unlock()

	d.notify()
}

// RemoveTool removes a tool by name at runtime and notifies listeners.
// Returns true if the tool existed.
func (d *DynamicRegistry) RemoveTool(name string) bool {
	d.mu.Lock()
	_, existed := d.tools[name]
	if existed {
		delete(d.tools, name)
		delete(d.deferred, name)
	}
	d.mu.Unlock()

	if existed {
		d.notify()
	}
	return existed
}

// RegisterModule registers a module and notifies listeners.
func (d *DynamicRegistry) RegisterModule(module ToolModule) {
	d.ToolRegistry.RegisterModule(module)
	d.notify()
}

// RegisterWithServer registers all tools with an MCP server and sets up
// the change notifier to emit tools/list_changed notifications via diff-based
// add/remove using WireToolListChanged.
func (d *DynamicRegistry) RegisterWithServer(s *MCPServer) {
	d.ToolRegistry.RegisterWithServer(s)
	WireToolListChanged(d, s)
}

// ToolFilter is a function that determines whether a tool should be visible
// in a given context (e.g., per-session, per-user, per-capability).
type ToolFilter func(td ToolDefinition) bool

// FilteredTools returns tool definitions that pass the filter.
func (r *ToolRegistry) FilteredTools(filter ToolFilter) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ToolDefinition
	for _, td := range r.tools {
		if filter(td) {
			result = append(result, td)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Tool.Name < result[j].Tool.Name
	})
	return result
}

// RegisterFilteredWithServer registers only tools passing the filter with an MCP server.
func (r *ToolRegistry) RegisterFilteredWithServer(s *MCPServer, filter ToolFilter) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		if !filter(tool) {
			continue
		}
		annotated := ApplyToolMetadata(tool, r.config.ToolNamePrefix, r.deferred[tool.Tool.Name])
		wrapped := r.wrapHandler(tool.Tool.Name, tool)
		AddToolToServer(s, annotated.Tool, wrapped)
	}
}

// ByCategory returns a ToolFilter that matches tools in the given category.
func ByCategory(category string) ToolFilter {
	return func(td ToolDefinition) bool {
		return td.Category == category
	}
}

// ByRuntimeGroup returns a ToolFilter matching tools in the given runtime group.
func ByRuntimeGroup(group string) ToolFilter {
	return func(td ToolDefinition) bool {
		return td.RuntimeGroup == group
	}
}

// ReadOnly returns a ToolFilter that only allows non-write tools.
func ReadOnly() ToolFilter {
	return func(td ToolDefinition) bool {
		return !td.IsWrite
	}
}

// Not inverts a ToolFilter.
func Not(filter ToolFilter) ToolFilter {
	return func(td ToolDefinition) bool {
		return !filter(td)
	}
}

// And combines multiple ToolFilters with AND logic.
func And(filters ...ToolFilter) ToolFilter {
	return func(td ToolDefinition) bool {
		for _, f := range filters {
			if !f(td) {
				return false
			}
		}
		return true
	}
}

// Exclude removes specific named tools.
func Exclude(names ...string) ToolFilter {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(td ToolDefinition) bool {
		return !set[td.Tool.Name]
	}
}

// NotDeferred filters tool names against a deferred set.
func NotDeferred(deferred map[string]bool) ToolFilter {
	return func(td ToolDefinition) bool {
		return !deferred[td.Tool.Name]
	}
}
