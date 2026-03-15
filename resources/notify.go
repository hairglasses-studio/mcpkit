//go:build !official_sdk

package resources

import (
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// WireResourceListChanged sets up an OnChange callback on the DynamicRegistry that
// diffs previous vs current resource/template sets and adds/removes items from
// the MCPServer accordingly.
func WireResourceListChanged(d *DynamicRegistry, s *registry.MCPServer) {
	var (
		mu       sync.Mutex
		prevRes  = make(map[string]bool)
		prevTmpl = make(map[string]bool)
	)

	// Capture initial state
	d.mu.RLock()
	for uri := range d.resources {
		prevRes[uri] = true
	}
	for tmplURI := range d.templates {
		prevTmpl[tmplURI] = true
	}
	d.mu.RUnlock()

	d.OnChange(func() {
		mu.Lock()
		defer mu.Unlock()

		d.mu.RLock()
		// Build current resource set
		currentRes := make(map[string]bool, len(d.resources))
		for uri := range d.resources {
			currentRes[uri] = true
		}

		// Add new resources
		for uri := range currentRes {
			if !prevRes[uri] {
				rd := d.resources[uri]
				wrapped := d.wrapHandler(rd.Resource.URI, rd)
				registry.AddResourceToServer(s, rd.Resource, wrapped)
			}
		}

		// Build current template set
		currentTmpl := make(map[string]bool, len(d.templates))
		for tmplURI := range d.templates {
			currentTmpl[tmplURI] = true
		}

		// Add new templates (no removal — mcp-go doesn't support it)
		for tmplURI := range currentTmpl {
			if !prevTmpl[tmplURI] {
				td := d.templates[tmplURI]
				wrapped := d.wrapTemplateHandler(td.Template.URITemplate.Raw(), td)
				registry.AddResourceTemplateToServer(s, td.Template, wrapped)
			}
		}
		d.mu.RUnlock()

		// Remove deleted resources
		var removed []string
		for uri := range prevRes {
			if !currentRes[uri] {
				removed = append(removed, uri)
			}
		}
		if len(removed) > 0 {
			registry.RemoveResourcesFromServer(s, removed...)
		}

		prevRes = currentRes
		prevTmpl = currentTmpl
	})
}
