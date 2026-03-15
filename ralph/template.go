package ralph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

// RenderSpec reads a spec file, applies Go text/template interpolation, then parses as JSON.
func RenderSpec(path string, vars map[string]string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: read spec template: %w", err)
	}
	return RenderSpecBytes(data, vars)
}

// RenderSpecBytes applies template interpolation to raw bytes, then parses as Spec.
func RenderSpecBytes(data []byte, vars map[string]string) (Spec, error) {
	tmpl, err := template.New("spec").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return Spec{}, fmt.Errorf("ralph: parse spec template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return Spec{}, fmt.Errorf("ralph: execute spec template: %w", err)
	}

	var spec Spec
	if err := json.Unmarshal(buf.Bytes(), &spec); err != nil {
		return Spec{}, fmt.Errorf("ralph: parse rendered spec: %w", err)
	}
	if err := ValidateSpec(spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}
