//go:build !official_sdk

package gateway

import (
	"context"
	"fmt"
	"sync"
)

// DynamicUpstreamRegistry provides runtime management of gateway upstreams
// with default resilience policies and lifecycle callbacks.
type DynamicUpstreamRegistry struct {
	mu       sync.RWMutex
	gateway  *Gateway
	configs  map[string]UpstreamConfig
	defaults UpstreamPolicy
	hooks    DynamicHooks
}

// DynamicHooks provides optional callbacks for upstream lifecycle events.
type DynamicHooks struct {
	OnAdd    func(name string, toolCount int)
	OnRemove func(name string)
}

// DynamicOption configures the DynamicUpstreamRegistry.
type DynamicOption func(*DynamicUpstreamRegistry)

// WithDefaultPolicy sets the default resilience policy for new upstreams.
func WithDefaultPolicy(p UpstreamPolicy) DynamicOption {
	return func(d *DynamicUpstreamRegistry) {
		d.defaults = p
	}
}

// WithDynamicHooks sets lifecycle callbacks.
func WithDynamicHooks(h DynamicHooks) DynamicOption {
	return func(d *DynamicUpstreamRegistry) {
		d.hooks = h
	}
}

// NewDynamicUpstreamRegistry creates a registry that manages upstreams for the given gateway.
func NewDynamicUpstreamRegistry(gw *Gateway, opts ...DynamicOption) *DynamicUpstreamRegistry {
	d := &DynamicUpstreamRegistry{
		gateway: gw,
		configs: make(map[string]UpstreamConfig),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Add registers a new upstream with the gateway. If the config has no Policy set,
// the registry's default policy is applied. Returns the number of tools discovered.
func (d *DynamicUpstreamRegistry) Add(ctx context.Context, cfg UpstreamConfig) (int, error) {
	if cfg.Name == "" {
		return 0, fmt.Errorf("gateway: upstream name cannot be empty")
	}

	d.mu.Lock()
	if _, exists := d.configs[cfg.Name]; exists {
		d.mu.Unlock()
		return 0, fmt.Errorf("%w: %s", ErrDuplicateUpstream, cfg.Name)
	}
	d.mu.Unlock()

	// Apply default policy if none set
	if cfg.Policy == (UpstreamPolicy{}) {
		cfg.Policy = d.defaults
	}

	count, err := d.gateway.AddUpstream(ctx, cfg)
	if err != nil {
		return 0, err
	}

	d.mu.Lock()
	d.configs[cfg.Name] = cfg
	d.mu.Unlock()

	if d.hooks.OnAdd != nil {
		d.hooks.OnAdd(cfg.Name, count)
	}

	return count, nil
}

// Remove deregisters an upstream and its tools from the gateway.
func (d *DynamicUpstreamRegistry) Remove(name string) error {
	d.mu.Lock()
	if _, exists := d.configs[name]; !exists {
		d.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrUpstreamNotFound, name)
	}
	delete(d.configs, name)
	d.mu.Unlock()

	d.gateway.RemoveUpstream(name)

	if d.hooks.OnRemove != nil {
		d.hooks.OnRemove(name)
	}

	return nil
}

// Get returns the config for a named upstream.
func (d *DynamicUpstreamRegistry) Get(name string) (UpstreamConfig, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cfg, ok := d.configs[name]
	return cfg, ok
}

// List returns the names of all registered upstreams.
func (d *DynamicUpstreamRegistry) List() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.configs))
	for name := range d.configs {
		names = append(names, name)
	}
	return names
}

// Len returns the number of registered upstreams.
func (d *DynamicUpstreamRegistry) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.configs)
}
