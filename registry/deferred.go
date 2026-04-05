package registry

// DeferredToolDefinition extends ToolDefinition with deferred loading support.
// When DeferLoading is true, the tool definition is discoverable via tool search
// but not loaded into the LLM context upfront — reducing token usage by up to 85%
// for servers with many tools.
//
// This implements the "Tool Search Tool" pattern from Anthropic's Advanced Tool Use:
// mark tools with defer_loading: true so they are only loaded on demand.
type DeferredToolDefinition struct {
	ToolDefinition

	// DeferLoading marks this tool for lazy loading. When true, the tool
	// is excluded from the initial tools/list response but remains discoverable
	// via search. Best for servers with 10+ tools.
	DeferLoading bool

	// SearchTerms are additional keywords for tool search discovery beyond
	// the tool's name, description, tags, and category. Use these for synonyms,
	// abbreviations, or domain-specific terms the LLM might search for.
	SearchTerms []string
}

// RegisterDeferredModule registers a module where tools can be individually
// marked for deferred loading. Tools with DeferLoading=true are stored but
// excluded from ListTools/RegisterWithServer until explicitly requested.
func (r *ToolRegistry) RegisterDeferredModule(module ToolModule, deferredTools map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.modules[module.Name()] = module

	for _, tool := range module.Tools() {
		if tool.RuntimeGroup == "" && r.config.RuntimeGroupMapper != nil {
			tool.RuntimeGroup = r.config.RuntimeGroupMapper(tool.Category)
		}
		if !tool.IsWrite {
			tool.IsWrite = InferIsWrite(tool.Tool.Name)
		}
		if deferredTools[tool.Tool.Name] {
			tool.DeferLoading = true
		}

		r.tools[tool.Tool.Name] = tool

		if deferredTools[tool.Tool.Name] {
			r.deferred[tool.Tool.Name] = true
		}
	}
}

// ListEagerTools returns only tools that are NOT deferred — the tools that
// should be loaded into the LLM context upfront.
func (r *ToolRegistry) ListEagerTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, tool := range r.tools {
		if !r.deferred[name] && !tool.DeferLoading {
			names = append(names, name)
		}
	}
	return names
}

// ListDeferredTools returns only tools marked for deferred loading.
func (r *ToolRegistry) ListDeferredTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, tool := range r.tools {
		if r.deferred[name] || tool.DeferLoading {
			names = append(names, name)
		}
	}
	return names
}

// SetDeferred marks or unmarks a tool for deferred loading.
func (r *ToolRegistry) SetDeferred(toolName string, deferred bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[toolName]
	if !exists {
		return
	}
	if deferred {
		r.deferred[toolName] = true
		tool.DeferLoading = true
	} else {
		delete(r.deferred, toolName)
		tool.DeferLoading = false
	}
	r.tools[toolName] = tool
}

// IsDeferred returns whether a tool is marked for deferred loading.
func (r *ToolRegistry) IsDeferred(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.deferred[toolName] {
		return true
	}
	tool, ok := r.tools[toolName]
	return ok && tool.DeferLoading
}
