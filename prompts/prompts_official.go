//go:build official_sdk

package prompts

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// PromptHandlerFunc is the function signature for prompt handlers.
type PromptHandlerFunc func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// Middleware wraps a prompt handler with additional behavior.
type Middleware func(name string, pd PromptDefinition, next PromptHandlerFunc) PromptHandlerFunc

// PromptDefinition represents a complete prompt with metadata.
type PromptDefinition struct {
	Prompt   mcp.Prompt
	Handler  PromptHandlerFunc
	Category string
	Tags     []string
}

// PromptModule is the interface that prompt modules implement.
type PromptModule interface {
	Name() string
	Description() string
	Prompts() []PromptDefinition
}

// DefaultGetTimeout is the maximum time a prompt get handler can run.
const DefaultGetTimeout = 30 * time.Second

// Config configures registry behavior.
type Config struct {
	DefaultTimeout time.Duration
	ListChanged    bool
	Middleware     []Middleware
}

// PromptRegistry manages prompt registration and lookup.
type PromptRegistry struct {
	mu      sync.RWMutex
	modules map[string]PromptModule
	prompts map[string]PromptDefinition
	config  Config
}

// NewPromptRegistry creates a new prompt registry.
func NewPromptRegistry(config ...Config) *PromptRegistry {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultGetTimeout
	}
	return &PromptRegistry{
		modules: make(map[string]PromptModule),
		prompts: make(map[string]PromptDefinition),
		config:  cfg,
	}
}

// RegisterModule registers a prompt module.
func (r *PromptRegistry) RegisterModule(module PromptModule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modules[module.Name()] = module
	for _, pd := range module.Prompts() {
		r.prompts[pd.Prompt.Name] = pd
	}
}

// GetPrompt returns a prompt definition by name.
func (r *PromptRegistry) GetPrompt(name string) (PromptDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pd, ok := r.prompts[name]
	return pd, ok
}

// GetModule returns a module by name.
func (r *PromptRegistry) GetModule(name string) (PromptModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[name]
	return module, ok
}

// ListPrompts returns all registered prompt names, sorted.
func (r *PromptRegistry) ListPrompts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.prompts))
	for name := range r.prompts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListPromptsByCategory returns prompt names filtered by category.
func (r *PromptRegistry) ListPromptsByCategory(category string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, pd := range r.prompts {
		if pd.Category == category {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// GetAllPromptDefinitions returns all registered prompt definitions.
func (r *PromptRegistry) GetAllPromptDefinitions() []PromptDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]PromptDefinition, 0, len(r.prompts))
	for _, pd := range r.prompts {
		all = append(all, pd)
	}
	return all
}

// PromptCount returns the number of registered prompts.
func (r *PromptRegistry) PromptCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.prompts)
}

// ModuleCount returns the number of registered modules.
func (r *PromptRegistry) ModuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// RegisterWithServer registers all prompts with an MCP server.
func (r *PromptRegistry) RegisterWithServer(s *registry.MCPServer) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, pd := range r.prompts {
		wrapped := r.wrapHandler(pd.Prompt.Name, pd)
		registry.AddPromptToServer(s, pd.Prompt, wrapped)
	}
}

func (r *PromptRegistry) wrapHandler(name string, pd PromptDefinition) PromptHandlerFunc {
	handler := pd.Handler
	for i := len(r.config.Middleware) - 1; i >= 0; i-- {
		handler = r.config.Middleware[i](name, pd, handler)
	}
	timeout := r.config.DefaultTimeout

	return func(ctx context.Context, request *mcp.GetPromptRequest) (result *mcp.GetPromptResult, err error) {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("panic in prompt %s: %v\n%s", name, rec, stack)
				result = nil
				slog.Error("prompt handler panicked", "prompt", name, "error", rec)
			}
		}()
		result, err = handler(ctx, request)
		if err != nil {
			slog.Error("prompt get failed", "prompt", name, "error", err)
		}
		return result, err
	}
}

// SearchPrompts performs a simple text search across prompt names, descriptions, and tags.
func (r *PromptRegistry) SearchPrompts(query string) []PromptDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	query = strings.ToLower(query)
	var results []PromptDefinition
	for _, pd := range r.prompts {
		if matchesPrompt(pd, query) {
			results = append(results, pd)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Prompt.Name < results[j].Prompt.Name
	})
	return results
}

func matchesPrompt(pd PromptDefinition, query string) bool {
	if strings.Contains(strings.ToLower(pd.Prompt.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(pd.Prompt.Description), query) {
		return true
	}
	if strings.EqualFold(pd.Category, query) {
		return true
	}
	for _, tag := range pd.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
