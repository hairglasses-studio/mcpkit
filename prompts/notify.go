//go:build !official_sdk

package prompts

import (
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// WirePromptListChanged sets up an OnChange callback on the DynamicRegistry that
// diffs previous vs current prompt sets and adds/removes prompts from the
// MCPServer accordingly.
func WirePromptListChanged(d *DynamicRegistry, s *registry.MCPServer) {
	var (
		mu   sync.Mutex
		prev = make(map[string]bool)
	)

	// Capture initial state
	d.mu.RLock()
	for name := range d.prompts {
		prev[name] = true
	}
	d.mu.RUnlock()

	d.OnChange(func() {
		mu.Lock()
		defer mu.Unlock()

		d.mu.RLock()
		current := make(map[string]bool, len(d.prompts))
		for name := range d.prompts {
			current[name] = true
		}

		// Add new prompts
		for name := range current {
			if !prev[name] {
				pd := d.prompts[name]
				wrapped := d.wrapHandler(pd.Prompt.Name, pd)
				registry.AddPromptToServer(s, pd.Prompt, wrapped)
			}
		}
		d.mu.RUnlock()

		// Remove deleted prompts
		var removed []string
		for name := range prev {
			if !current[name] {
				removed = append(removed, name)
			}
		}
		if len(removed) > 0 {
			registry.RemovePromptsFromServer(s, removed...)
		}

		prev = current
	})
}
