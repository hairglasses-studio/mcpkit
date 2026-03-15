package extensions

import (
	"fmt"
	"sync"
)

// Extension declares an optional capability.
type Extension struct {
	Name        string            // e.g. "mcpkit:health", "mcpkit:finops"
	Version     string            // semver, e.g. "1.0.0"
	Description string
	Required    bool              // if true, server won't start without client support
	Metadata    map[string]string // arbitrary key-value for capability details
}

// NegotiationResult records the outcome for one extension.
type NegotiationResult struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// ExtensionRegistry manages available and negotiated extensions.
type ExtensionRegistry struct {
	mu         sync.RWMutex
	extensions map[string]Extension
	active     map[string]bool
}

// NewExtensionRegistry creates a new empty registry.
func NewExtensionRegistry() *ExtensionRegistry {
	return &ExtensionRegistry{
		extensions: make(map[string]Extension),
		active:     make(map[string]bool),
	}
}

// Register adds an extension to the registry. Returns error if name is empty or duplicate.
func (r *ExtensionRegistry) Register(ext Extension) error {
	if ext.Name == "" {
		return fmt.Errorf("extensions: name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.extensions[ext.Name]; exists {
		return fmt.Errorf("extensions: duplicate extension %q", ext.Name)
	}
	r.extensions[ext.Name] = ext
	return nil
}

// Available returns all registered extensions.
func (r *ExtensionRegistry) Available() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Extension, 0, len(r.extensions))
	for _, ext := range r.extensions {
		result = append(result, ext)
	}
	return result
}

// Negotiate takes a list of extension names offered by the client and returns
// negotiation results. Offered extensions that match registered ones are accepted.
// Required extensions not in the offered list are rejected with a reason.
func (r *ExtensionRegistry) Negotiate(offered []string) []NegotiationResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	offeredSet := make(map[string]bool, len(offered))
	for _, name := range offered {
		offeredSet[name] = true
	}

	var results []NegotiationResult
	for name, ext := range r.extensions {
		if offeredSet[name] {
			r.active[name] = true
			results = append(results, NegotiationResult{
				Name:     name,
				Version:  ext.Version,
				Accepted: true,
			})
		} else if ext.Required {
			results = append(results, NegotiationResult{
				Name:     name,
				Version:  ext.Version,
				Accepted: false,
				Reason:   "required extension not offered by client",
			})
		}
	}
	return results
}

// IsActive returns true if the named extension was successfully negotiated.
func (r *ExtensionRegistry) IsActive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active[name]
}

// Active returns all extensions that have been successfully negotiated.
func (r *ExtensionRegistry) Active() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Extension
	for name := range r.active {
		if ext, ok := r.extensions[name]; ok {
			result = append(result, ext)
		}
	}
	return result
}
