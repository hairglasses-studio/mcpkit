//go:build !official_sdk

package resources

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

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ResourceHandlerFunc is the function signature for resource read handlers.
type ResourceHandlerFunc func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

// Middleware wraps a resource handler with additional behavior.
// It receives the resource URI, definition, and next handler in the chain.
type Middleware func(uri string, rd ResourceDefinition, next ResourceHandlerFunc) ResourceHandlerFunc

// ResourceDefinition represents a complete resource with metadata.
type ResourceDefinition struct {
	Resource mcp.Resource
	Handler  ResourceHandlerFunc
	Category string
	Tags     []string
	// CacheTTL is how long clients should cache this resource. Zero means no caching hint.
	CacheTTL time.Duration
}

// TemplateDefinition represents a resource template with metadata.
type TemplateDefinition struct {
	Template mcp.ResourceTemplate
	Handler  ResourceHandlerFunc
	Category string
	Tags     []string
}

// ResourceModule is the interface that resource modules implement.
type ResourceModule interface {
	Name() string
	Description() string
	Resources() []ResourceDefinition
	Templates() []TemplateDefinition
}

// DefaultReadTimeout is the maximum time a resource read handler can run.
const DefaultReadTimeout = 30 * time.Second

// Config configures registry behavior.
type Config struct {
	// DefaultTimeout for resource read handlers. Zero uses DefaultReadTimeout (30s).
	DefaultTimeout time.Duration

	// Subscribe enables resource subscription support.
	Subscribe bool

	// ListChanged enables list-changed notifications.
	ListChanged bool

	// Middleware to apply to all handlers, in order (outermost first).
	Middleware []Middleware
}

// ResourceRegistry manages resource registration and lookup.
type ResourceRegistry struct {
	mu        sync.RWMutex
	modules   map[string]ResourceModule
	resources map[string]ResourceDefinition // keyed by URI
	templates map[string]TemplateDefinition // keyed by URI template
	config    Config
}

// NewResourceRegistry creates a new resource registry with the given config.
func NewResourceRegistry(config ...Config) *ResourceRegistry {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultReadTimeout
	}
	return &ResourceRegistry{
		modules:   make(map[string]ResourceModule),
		resources: make(map[string]ResourceDefinition),
		templates: make(map[string]TemplateDefinition),
		config:    cfg,
	}
}

// RegisterModule registers a resource module with the registry.
func (r *ResourceRegistry) RegisterModule(module ResourceModule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.modules[module.Name()] = module

	for _, res := range module.Resources() {
		r.resources[res.Resource.URI] = res
	}
	for _, tmpl := range module.Templates() {
		r.templates[tmpl.Template.URITemplate.Raw()] = tmpl
	}
}

// GetResource returns a resource definition by URI.
func (r *ResourceRegistry) GetResource(uri string) (ResourceDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rd, ok := r.resources[uri]
	return rd, ok
}

// GetTemplate returns a template definition by URI template string.
func (r *ResourceRegistry) GetTemplate(uriTemplate string) (TemplateDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	td, ok := r.templates[uriTemplate]
	return td, ok
}

// GetModule returns a module by name.
func (r *ResourceRegistry) GetModule(name string) (ResourceModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[name]
	return module, ok
}

// ListResources returns all registered resource URIs, sorted.
func (r *ResourceRegistry) ListResources() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	uris := make([]string, 0, len(r.resources))
	for uri := range r.resources {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}

// ListTemplates returns all registered template URI patterns, sorted.
func (r *ResourceRegistry) ListTemplates() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	patterns := make([]string, 0, len(r.templates))
	for pattern := range r.templates {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	return patterns
}

// ListResourcesByCategory returns resource URIs filtered by category, sorted.
func (r *ResourceRegistry) ListResourcesByCategory(category string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var uris []string
	for uri, rd := range r.resources {
		if rd.Category == category {
			uris = append(uris, uri)
		}
	}
	sort.Strings(uris)
	return uris
}

// GetAllResourceDefinitions returns all registered resource definitions.
func (r *ResourceRegistry) GetAllResourceDefinitions() []ResourceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]ResourceDefinition, 0, len(r.resources))
	for _, rd := range r.resources {
		all = append(all, rd)
	}
	return all
}

// GetAllTemplateDefinitions returns all registered template definitions.
func (r *ResourceRegistry) GetAllTemplateDefinitions() []TemplateDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]TemplateDefinition, 0, len(r.templates))
	for _, td := range r.templates {
		all = append(all, td)
	}
	return all
}

// ResourceCount returns the number of registered resources.
func (r *ResourceRegistry) ResourceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.resources)
}

// TemplateCount returns the number of registered templates.
func (r *ResourceRegistry) TemplateCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.templates)
}

// ModuleCount returns the number of registered modules.
func (r *ResourceRegistry) ModuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// RegisterWithServer registers all resources and templates with an MCP server,
// applying the configured middleware chain.
func (r *ResourceRegistry) RegisterWithServer(s *registry.MCPServer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rd := range r.resources {
		wrapped := r.wrapHandler(rd.Resource.URI, rd)
		registry.AddResourceToServer(s, rd.Resource, wrapped)
	}

	for _, td := range r.templates {
		wrapped := r.wrapTemplateHandler(td.Template.URITemplate.Raw(), td)
		registry.AddResourceTemplateToServer(s, td.Template, wrapped)
	}
}

// wrapHandler applies middleware, timeout, and panic recovery to a resource handler.
func (r *ResourceRegistry) wrapHandler(uri string, rd ResourceDefinition) ResourceHandlerFunc {
	handler := rd.Handler

	// Apply user-configured middleware (innermost applied first, so iterate in reverse)
	for i := len(r.config.Middleware) - 1; i >= 0; i-- {
		handler = r.config.Middleware[i](uri, rd, handler)
	}

	timeout := r.config.DefaultTimeout

	return func(ctx context.Context, request mcp.ReadResourceRequest) (result []mcp.ResourceContents, err error) {
		// Enforce timeout
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("panic reading resource %s: %v\n%s", uri, rec, stack)
				result = nil
				slog.Error("resource handler panicked", "uri", uri, "error", rec)
			}
		}()

		result, err = handler(ctx, request)

		if err != nil {
			slog.Error("resource read failed", "uri", uri, "error", err)
		}

		return result, err
	}
}

// wrapTemplateHandler applies middleware, timeout, and panic recovery to a template handler.
func (r *ResourceRegistry) wrapTemplateHandler(uriTemplate string, td TemplateDefinition) ResourceHandlerFunc {
	handler := td.Handler

	// Templates reuse the same middleware chain — wrap with a synthetic ResourceDefinition
	rd := ResourceDefinition{
		Resource: mcp.Resource{
			URI:         uriTemplate,
			Name:        td.Template.Name,
			Description: td.Template.Description,
			MIMEType:    td.Template.MIMEType,
		},
		Handler:  handler,
		Category: td.Category,
		Tags:     td.Tags,
	}

	for i := len(r.config.Middleware) - 1; i >= 0; i-- {
		handler = r.config.Middleware[i](uriTemplate, rd, handler)
	}

	timeout := r.config.DefaultTimeout

	return func(ctx context.Context, request mcp.ReadResourceRequest) (result []mcp.ResourceContents, err error) {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				err = fmt.Errorf("panic reading resource template %s: %v\n%s", uriTemplate, rec, stack)
				result = nil
				slog.Error("resource template handler panicked", "uri_template", uriTemplate, "error", rec)
			}
		}()

		result, err = handler(ctx, request)

		if err != nil {
			slog.Error("resource template read failed", "uri_template", uriTemplate, "error", err)
		}

		return result, err
	}
}

// SearchResources performs a simple text search across resource URIs, names, descriptions, and tags.
func (r *ResourceRegistry) SearchResources(query string) []ResourceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	var results []ResourceDefinition

	for _, rd := range r.resources {
		if matchesResource(rd, query) {
			results = append(results, rd)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Resource.URI < results[j].Resource.URI
	})
	return results
}

func matchesResource(rd ResourceDefinition, query string) bool {
	if strings.Contains(strings.ToLower(rd.Resource.URI), query) {
		return true
	}
	if strings.Contains(strings.ToLower(rd.Resource.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(rd.Resource.Description), query) {
		return true
	}
	if strings.EqualFold(rd.Category, query) {
		return true
	}
	for _, tag := range rd.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
