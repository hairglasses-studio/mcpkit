package registry

import "testing"

// TestTemplateURI exercises the real consumer pattern (secretstudios-mcp's
// internal/surface/server_catalog.go, which needs the raw template string
// for display/sorting): construct via NewResourceTemplate, then read it back
// via TemplateURI instead of reaching into the SDK-specific URITemplate
// field directly. Runs unmodified under both build tags.
func TestTemplateURI(t *testing.T) {
	tpl := NewResourceTemplate("test://dynamic/{name}", "Dynamic Resource")
	got := TemplateURI(tpl)
	if got != "test://dynamic/{name}" {
		t.Errorf("TemplateURI = %q, want test://dynamic/{name}", got)
	}
}

func TestTemplateURI_Zero(t *testing.T) {
	var tpl ResourceTemplate
	if got := TemplateURI(tpl); got != "" {
		t.Errorf("TemplateURI(zero value) = %q, want empty", got)
	}
}
