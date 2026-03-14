// Package registry provides the core tool registry and interfaces for MCP servers.
//
// It manages tool registration, lookup, search, and middleware-based handler wrapping.
// Tool modules implement the ToolModule interface and register via RegisterModule.
package registry

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolHandlerFunc is the function signature for tool handlers.
type ToolHandlerFunc func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Middleware wraps a tool handler with additional behavior.
// It receives the tool name, definition, and next handler in the chain.
type Middleware func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc

// ToolComplexity indicates the complexity level of a tool.
type ToolComplexity string

const (
	ComplexitySimple   ToolComplexity = "simple"
	ComplexityModerate ToolComplexity = "moderate"
	ComplexityComplex  ToolComplexity = "complex"
)

// ToolDefinition represents a complete tool with metadata.
type ToolDefinition struct {
	Tool                mcp.Tool
	Handler             ToolHandlerFunc
	Category            string
	Subcategory         string
	Tags                []string
	UseCases            []string
	Complexity          ToolComplexity
	IsWrite             bool
	Deprecated          bool
	Successor           string
	Timeout             time.Duration
	CircuitBreakerGroup string
	RuntimeGroup        string
	OutputSchema        *mcp.ToolOutputSchema
}

// ToolModule is the interface that tool modules implement.
type ToolModule interface {
	Name() string
	Description() string
	Tools() []ToolDefinition
}

// DefaultToolTimeout is the maximum time a tool handler can run.
const DefaultToolTimeout = 30 * time.Second

// DefaultMaxResponseSize is the maximum response size before truncation (128KB).
const DefaultMaxResponseSize = 128 * 1024

// Config configures registry behavior.
type Config struct {
	// DefaultTimeout for tool handlers. Zero uses DefaultToolTimeout (30s).
	DefaultTimeout time.Duration

	// MaxResponseSize for truncation. Zero uses DefaultMaxResponseSize (128KB).
	MaxResponseSize int

	// ToolNamePrefix to strip when generating annotation titles (e.g., "myapp_").
	ToolNamePrefix string

	// RuntimeGroupMapper maps a category to a runtime group.
	// If nil or returns empty string, RuntimeGroup is left as-is from the ToolDefinition.
	RuntimeGroupMapper func(category string) string

	// Middleware to apply to all handlers, in order (outermost first).
	Middleware []Middleware
}

// ToolRegistry manages tool registration and lookup.
type ToolRegistry struct {
	mu       sync.RWMutex
	modules  map[string]ToolModule
	tools    map[string]ToolDefinition
	deferred map[string]bool // tools marked for deferred/lazy loading
	config   Config
}

// NewToolRegistry creates a new tool registry with the given config.
func NewToolRegistry(config ...Config) *ToolRegistry {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultToolTimeout
	}
	if cfg.MaxResponseSize == 0 {
		cfg.MaxResponseSize = DefaultMaxResponseSize
	}
	return &ToolRegistry{
		modules:  make(map[string]ToolModule),
		tools:    make(map[string]ToolDefinition),
		deferred: make(map[string]bool),
		config:   cfg,
	}
}

// RegisterModule registers a tool module with the registry.
func (r *ToolRegistry) RegisterModule(module ToolModule) {
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
		r.tools[tool.Tool.Name] = tool
	}
}

// GetTool returns a tool definition by name.
func (r *ToolRegistry) GetTool(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// GetModule returns a module by name.
func (r *ToolRegistry) GetModule(name string) (ToolModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[name]
	return module, ok
}

// ListModules returns all registered module names, sorted.
func (r *ToolRegistry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListTools returns all registered tool names, sorted.
func (r *ToolRegistry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListToolsByCategory returns tools filtered by category, sorted.
func (r *ToolRegistry) ListToolsByCategory(category string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, tool := range r.tools {
		if tool.Category == category {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ListToolsByRuntimeGroup returns tools filtered by runtime group, sorted.
func (r *ToolRegistry) ListToolsByRuntimeGroup(group string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, tool := range r.tools {
		if tool.RuntimeGroup == group {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// GetRuntimeGroupStats returns tool counts per runtime group.
func (r *ToolRegistry) GetRuntimeGroupStats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := make(map[string]int)
	for _, tool := range r.tools {
		group := tool.RuntimeGroup
		if group == "" {
			group = "unassigned"
		}
		stats[group]++
	}
	return stats
}

// GetAllToolDefinitions returns all registered tool definitions.
func (r *ToolRegistry) GetAllToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	allTools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		allTools = append(allTools, tool)
	}
	return allTools
}

// ToolCount returns the number of registered tools.
func (r *ToolRegistry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// ModuleCount returns the number of registered modules.
func (r *ToolRegistry) ModuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// ToolStats holds statistics about registered tools.
type ToolStats struct {
	TotalTools      int            `json:"total_tools"`
	ModuleCount     int            `json:"module_count"`
	ByCategory      map[string]int `json:"by_category"`
	ByComplexity    map[string]int `json:"by_complexity"`
	ByRuntimeGroup  map[string]int `json:"by_runtime_group"`
	WriteToolsCount int            `json:"write_tools_count"`
	ReadOnlyCount   int            `json:"read_only_count"`
	DeprecatedCount int            `json:"deprecated_count"`
}

// GetToolStats returns statistics about the registered tools.
func (r *ToolRegistry) GetToolStats() ToolStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := ToolStats{
		TotalTools:     len(r.tools),
		ModuleCount:    len(r.modules),
		ByCategory:     make(map[string]int),
		ByComplexity:   make(map[string]int),
		ByRuntimeGroup: make(map[string]int),
	}
	for _, tool := range r.tools {
		stats.ByCategory[tool.Category]++
		stats.ByComplexity[string(tool.Complexity)]++
		group := tool.RuntimeGroup
		if group == "" {
			group = "unassigned"
		}
		stats.ByRuntimeGroup[group]++
		if tool.IsWrite {
			stats.WriteToolsCount++
		} else {
			stats.ReadOnlyCount++
		}
		if tool.Deprecated {
			stats.DeprecatedCount++
		}
	}
	return stats
}

// GetToolCatalog returns a structured catalog of all tools organized by category/subcategory.
func (r *ToolRegistry) GetToolCatalog() map[string]map[string][]ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	catalog := make(map[string]map[string][]ToolDefinition)
	for _, tool := range r.tools {
		if catalog[tool.Category] == nil {
			catalog[tool.Category] = make(map[string][]ToolDefinition)
		}
		subcategory := tool.Subcategory
		if subcategory == "" {
			subcategory = "general"
		}
		catalog[tool.Category][subcategory] = append(catalog[tool.Category][subcategory], tool)
	}
	return catalog
}

// RegisterWithServer registers all tools with an MCP server, applying
// annotations, output schemas, and the configured middleware chain.
func (r *ToolRegistry) RegisterWithServer(s *MCPServer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		annotated := ApplyMCPAnnotations(tool, r.config.ToolNamePrefix)
		if annotated.OutputSchema != nil {
			annotated.Tool.OutputSchema = *annotated.OutputSchema
		}
		wrapped := r.wrapHandler(tool.Tool.Name, tool)
		AddToolToServer(s, annotated.Tool, wrapped)
	}
}

// wrapHandler applies the built-in middleware (timeout, panic recovery, truncation)
// and any configured middleware chain.
func (r *ToolRegistry) wrapHandler(toolName string, td ToolDefinition) ToolHandlerFunc {
	handler := td.Handler

	// Apply user-configured middleware (innermost applied first, so iterate in reverse)
	for i := len(r.config.Middleware) - 1; i >= 0; i-- {
		handler = r.config.Middleware[i](toolName, td, handler)
	}

	timeout := td.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}
	maxSize := r.config.MaxResponseSize

	return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		// Enforce timeout
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Panic recovery
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("panic in %s: %v\n%s", toolName, r, stack)
				result = MakeErrorResult(fmt.Sprintf("Internal error in %s: recovered from panic", toolName))
				slog.Error("tool panicked", "tool", toolName, "error", r)
			}
		}()

		result, err = handler(ctx, request)

		// Truncate oversized responses
		result = truncateResponse(result, maxSize)

		// Log errors
		if err != nil {
			slog.Error("tool failed", "tool", toolName, "error", err)
		} else if IsResultError(result) {
			for _, content := range result.Content {
				if text, ok := ExtractTextContent(content); ok && len(text) > 1 && text[0] == '[' {
					if idx := strings.Index(text, "]"); idx > 0 {
						code := text[1:idx]
						slog.Warn("tool error", "tool", toolName, "error_code", code)
					}
				}
			}
		}

		return result, err
	}
}

// truncateResponse truncates text content exceeding maxSize.
func truncateResponse(result *mcp.CallToolResult, maxSize int) *mcp.CallToolResult {
	if result == nil || maxSize <= 0 {
		return result
	}
	for i, content := range result.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			if len(tc.Text) > maxSize {
				tc.Text = tc.Text[:maxSize] + fmt.Sprintf("\n\n[TRUNCATED: response exceeded %dKB limit]", maxSize/1024)
				result.Content[i] = tc
			}
		}
	}
	return result
}
