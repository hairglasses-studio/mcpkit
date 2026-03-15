package ralph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// RenderSpec reads a spec file, applies Go text/template interpolation, then parses as JSON or YAML.
// Files ending in .yaml or .yml are parsed as YAML after template rendering; all others are parsed as JSON.
func RenderSpec(path string, vars map[string]string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: read spec template: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	return renderSpecBytes(data, vars, ext)
}

// RenderSpecBytes applies template interpolation to raw bytes, then parses as Spec.
// Format is detected from content: valid JSON is parsed as JSON, otherwise YAML is attempted.
// This function maintains backwards compatibility — JSON remains the default.
func RenderSpecBytes(data []byte, vars map[string]string) (Spec, error) {
	return renderSpecBytes(data, vars, "")
}

// renderSpecBytes is the internal implementation that accepts an explicit extension hint.
// When ext is ".yaml" or ".yml", YAML parsing is used after template rendering.
// Otherwise JSON is attempted first; if it fails and the content looks like YAML, YAML is tried.
func renderSpecBytes(data []byte, vars map[string]string, ext string) (Spec, error) {
	tmpl, err := template.New("spec").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: parse spec template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return Spec{}, fmt.Errorf("ralph: execute spec template: %w", err)
	}

	rendered := buf.Bytes()

	// If an explicit YAML extension was provided, use YAML parsing.
	if ext == ".yaml" || ext == ".yml" {
		var spec Spec
		if err := yaml.Unmarshal(rendered, &spec); err != nil {
			return Spec{}, fmt.Errorf("ralph: parse rendered yaml spec: %w", err)
		}
		if err := ValidateSpec(spec); err != nil {
			return Spec{}, err
		}
		return spec, nil
	}

	// Default: JSON parsing.
	var spec Spec
	if err := json.Unmarshal(rendered, &spec); err != nil {
		return Spec{}, fmt.Errorf("ralph: parse rendered spec: %w", err)
	}
	if err := ValidateSpec(spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}
