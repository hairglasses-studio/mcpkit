package ralph

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadSpecYAML reads a YAML spec file from disk.
func LoadSpecYAML(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read yaml spec: %w", err)
	}
	return ParseSpecYAML(data)
}

// ParseSpecYAML unmarshals YAML bytes into a Spec and validates it.
func ParseSpecYAML(data []byte) (Spec, error) {
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Spec{}, fmt.Errorf("parse yaml spec: %w", err)
	}
	if err := ValidateSpec(s); err != nil {
		return Spec{}, err
	}
	return s, nil
}
