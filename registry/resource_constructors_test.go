package registry

import "testing"

// TestNewResource_TwoArgPlusFieldAssignment exercises the exact pattern real
// consumers use (secretstudios-mcp's internal/surface/resources.go
// newDocResource helper): call NewResource(uri, name) with no options, then
// set Description/MIMEType directly on the returned value. Runs unmodified
// under both build tags (no SDK-specific import) since Resource is a
// per-tag alias with the same exported field names on both.
func TestNewResource_TwoArgPlusFieldAssignment(t *testing.T) {
	r := NewResource("test://doc", "Test Doc")
	r.Description = "a test resource"
	r.MIMEType = "text/markdown"

	if r.URI != "test://doc" {
		t.Errorf("URI = %q, want test://doc", r.URI)
	}
	if r.Name != "Test Doc" {
		t.Errorf("Name = %q, want Test Doc", r.Name)
	}
	if r.Description != "a test resource" {
		t.Errorf("Description = %q, want 'a test resource'", r.Description)
	}
	if r.MIMEType != "text/markdown" {
		t.Errorf("MIMEType = %q, want text/markdown", r.MIMEType)
	}
}

// TestNewResourceTemplate_TwoArgPlusFieldAssignment mirrors
// TestNewResource_TwoArgPlusFieldAssignment for the template constructor
// (secretstudios-mcp's newJSONTemplate helper). Deliberately does not assert
// on the URITemplate field itself — mcp-go's is a parsed *mcp.URITemplate,
// the official SDK's is a plain string, so it isn't a portable assertion in
// a neutral (no build tag) test file; Name/Description/MIMEType are
// symmetric on both and are what real consumers actually read back.
func TestNewResourceTemplate_TwoArgPlusFieldAssignment(t *testing.T) {
	tpl := NewResourceTemplate("test://template/{id}", "Test Template")
	tpl.Description = "a test template"
	tpl.MIMEType = "application/json"

	if tpl.Name != "Test Template" {
		t.Errorf("Name = %q, want Test Template", tpl.Name)
	}
	if tpl.Description != "a test template" {
		t.Errorf("Description = %q, want 'a test template'", tpl.Description)
	}
	if tpl.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want application/json", tpl.MIMEType)
	}
}
