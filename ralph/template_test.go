package ralph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSpec_WithVars(t *testing.T) {
	dir := t.TempDir()
	tmplFile := filepath.Join(dir, "spec.json")
	content := `{
		"name": "deploy-{{.Service}}",
		"description": "Deploy {{.Service}} to {{.Env}}",
		"completion": "{{.Service}} is deployed",
		"tasks": [
			{"id": "t1", "description": "Build {{.Service}}"},
			{"id": "t2", "description": "Deploy to {{.Env}}"}
		]
	}`
	os.WriteFile(tmplFile, []byte(content), 0644)

	spec, err := RenderSpec(tmplFile, map[string]string{
		"Service": "api-gateway",
		"Env":     "staging",
	})
	if err != nil {
		t.Fatalf("RenderSpec: %v", err)
	}
	if spec.Name != "deploy-api-gateway" {
		t.Errorf("Name = %q, want %q", spec.Name, "deploy-api-gateway")
	}
	if !strings.Contains(spec.Description, "staging") {
		t.Errorf("Description = %q, want mention of 'staging'", spec.Description)
	}
	if len(spec.Tasks) != 2 {
		t.Errorf("Tasks len = %d, want 2", len(spec.Tasks))
	}
}

func TestRenderSpec_MissingVar(t *testing.T) {
	dir := t.TempDir()
	tmplFile := filepath.Join(dir, "spec.json")
	content := `{"name": "{{.Service}}", "description": "test", "tasks": [{"id": "t1", "description": "do"}]}`
	os.WriteFile(tmplFile, []byte(content), 0644)

	_, err := RenderSpec(tmplFile, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing template variable")
	}
}

func TestRenderSpec_NoVars(t *testing.T) {
	dir := t.TempDir()
	tmplFile := filepath.Join(dir, "spec.json")
	content := `{"name": "plain", "description": "no templates", "tasks": [{"id": "t1", "description": "do it"}]}`
	os.WriteFile(tmplFile, []byte(content), 0644)

	spec, err := RenderSpec(tmplFile, map[string]string{})
	if err != nil {
		t.Fatalf("RenderSpec: %v", err)
	}
	if spec.Name != "plain" {
		t.Errorf("Name = %q, want %q", spec.Name, "plain")
	}
}

func TestRenderSpec_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	tmplFile := filepath.Join(dir, "spec.json")
	content := `{"name": "{{.Broken", "description": "test"}`
	os.WriteFile(tmplFile, []byte(content), 0644)

	_, err := RenderSpec(tmplFile, map[string]string{})
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
	if !strings.Contains(err.Error(), "parse spec template") {
		t.Errorf("error = %q, want mention of 'parse spec template'", err)
	}
}

func TestRenderSpecBytes_WithVars(t *testing.T) {
	data := []byte(`{"name": "test-{{.Name}}", "description": "desc", "tasks": [{"id": "t1", "description": "do"}]}`)
	spec, err := RenderSpecBytes(data, map[string]string{"Name": "foo"})
	if err != nil {
		t.Fatalf("RenderSpecBytes: %v", err)
	}
	if spec.Name != "test-foo" {
		t.Errorf("Name = %q, want %q", spec.Name, "test-foo")
	}
}

func TestRenderSpec_FileNotFound(t *testing.T) {
	_, err := RenderSpec("/nonexistent/spec.json", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
