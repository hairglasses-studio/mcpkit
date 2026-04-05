//go:build !official_sdk

package registry

import "sync"

// WireToolListChanged sets up an OnChange callback on the DynamicRegistry that
// diffs previous vs current tool sets and adds/removes tools from the MCPServer
// accordingly. This ensures tools/list_changed notifications fire for both
// additions and removals.
func WireToolListChanged(d *DynamicRegistry, s *MCPServer) {
	var (
		mu   sync.Mutex
		prev = make(map[string]bool)
	)

	// Capture initial tool set
	d.mu.RLock()
	for name := range d.tools {
		prev[name] = true
	}
	d.mu.RUnlock()

	d.OnChange(func() {
		mu.Lock()
		defer mu.Unlock()

		// Build current set
		d.mu.RLock()
		current := make(map[string]bool, len(d.tools))
		for name := range d.tools {
			current[name] = true
		}

		// Find added tools and re-register them
		for name := range current {
			if !prev[name] {
				td := d.tools[name]
				annotated := ApplyToolMetadata(td, d.config.ToolNamePrefix, d.deferred[name])
				wrapped := d.wrapHandler(td.Tool.Name, td)
				AddToolToServer(s, annotated.Tool, wrapped)
			}
		}
		d.mu.RUnlock()

		// Find removed tools
		var removed []string
		for name := range prev {
			if !current[name] {
				removed = append(removed, name)
			}
		}
		if len(removed) > 0 {
			RemoveToolsFromServer(s, removed...)
		}

		// Update prev
		prev = current
	})
}
