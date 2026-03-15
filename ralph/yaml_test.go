package ralph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const validYAMLSpec = `
name: test-spec
description: A test specification
completion: All tasks done
tasks:
  - id: task1
    description: First task
  - id: task2
    description: Second task
`

const yamlSpecWithDeps = `
name: dep-spec
description: Spec with dependencies
completion: done
tasks:
  - id: alpha
    description: Alpha task
  - id: beta
    description: Beta task
    depends_on:
      - alpha
  - id: gamma
    description: Gamma task
    depends_on:
      - alpha
      - beta
`

const invalidYAML = `
name: [bad yaml
description: this is broken
  tasks: oops
`

func TestLoadSpecYAML_Valid(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "spec-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(validYAMLSpec); err != nil {
		t.Fatal(err)
	}
	f.Close()

	spec, err := LoadSpecYAML(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "test-spec" {
		t.Errorf("Name = %q, want %q", spec.Name, "test-spec")
	}
	if spec.Description != "A test specification" {
		t.Errorf("Description = %q, want %q", spec.Description, "A test specification")
	}
	if spec.Completion != "All tasks done" {
		t.Errorf("Completion = %q, want %q", spec.Completion, "All tasks done")
	}
	if len(spec.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(spec.Tasks))
	}
	if spec.Tasks[0].ID != "task1" {
		t.Errorf("Tasks[0].ID = %q, want %q", spec.Tasks[0].ID, "task1")
	}
	if spec.Tasks[1].ID != "task2" {
		t.Errorf("Tasks[1].ID = %q, want %q", spec.Tasks[1].ID, "task2")
	}
}

func TestLoadSpecYAML_WithDependencies(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "spec-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(yamlSpecWithDeps); err != nil {
		t.Fatal(err)
	}
	f.Close()

	spec, err := LoadSpecYAML(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(spec.Tasks))
	}

	// alpha has no deps
	if len(spec.Tasks[0].DependsOn) != 0 {
		t.Errorf("alpha DependsOn = %v, want empty", spec.Tasks[0].DependsOn)
	}
	// beta depends on alpha
	if len(spec.Tasks[1].DependsOn) != 1 || spec.Tasks[1].DependsOn[0] != "alpha" {
		t.Errorf("beta DependsOn = %v, want [alpha]", spec.Tasks[1].DependsOn)
	}
	// gamma depends on alpha and beta
	if len(spec.Tasks[2].DependsOn) != 2 {
		t.Errorf("gamma DependsOn = %v, want [alpha beta]", spec.Tasks[2].DependsOn)
	}
}

func TestLoadSpecYAML_InvalidYAML(t *testing.T) {
	_, err := ParseSpecYAML([]byte(invalidYAML))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadSpecYAML_MissingFile(t *testing.T) {
	_, err := LoadSpecYAML("/nonexistent/path/spec.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadSpec_DetectsYAMLExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myspec.yaml")
	if err := os.WriteFile(path, []byte(validYAMLSpec), 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec(.yaml) error: %v", err)
	}
	if spec.Name != "test-spec" {
		t.Errorf("Name = %q, want %q", spec.Name, "test-spec")
	}
	if len(spec.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(spec.Tasks))
	}
}

func TestLoadSpec_DetectsYMLExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myspec.yml")
	if err := os.WriteFile(path, []byte(validYAMLSpec), 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec(.yml) error: %v", err)
	}
	if spec.Name != "test-spec" {
		t.Errorf("Name = %q, want %q", spec.Name, "test-spec")
	}
}

const yamlTemplateSpec = `
name: {{ index . "project" }}
description: Spec for {{ index . "project" }}
completion: done
tasks:
  - id: build
    description: Build {{ index . "project" }}
`

func TestRenderSpec_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.yaml")
	if err := os.WriteFile(path, []byte(yamlTemplateSpec), 0o600); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{"project": "myapp"}
	spec, err := RenderSpec(path, vars)
	if err != nil {
		t.Fatalf("RenderSpec(.yaml) error: %v", err)
	}
	if spec.Name != "myapp" {
		t.Errorf("Name = %q, want %q", spec.Name, "myapp")
	}
	if spec.Tasks[0].Description != "Build myapp" {
		t.Errorf("Tasks[0].Description = %q, want %q", spec.Tasks[0].Description, "Build myapp")
	}
}

func TestParseSpecYAML_EquivalentToJSON(t *testing.T) {
	jsonSpec := `{
		"name": "equiv-spec",
		"description": "Equivalence test",
		"completion": "done",
		"tasks": [
			{"id": "t1", "description": "Task one"},
			{"id": "t2", "description": "Task two", "depends_on": ["t1"]}
		]
	}`

	yamlSpec := `
name: equiv-spec
description: Equivalence test
completion: done
tasks:
  - id: t1
    description: Task one
  - id: t2
    description: Task two
    depends_on:
      - t1
`

	// Parse JSON via LoadSpec path
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(jsonPath, []byte(jsonSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	specFromJSON, err := LoadSpec(jsonPath)
	if err != nil {
		t.Fatalf("LoadSpec(json) error: %v", err)
	}

	// Parse YAML via ParseSpecYAML
	specFromYAML, err := ParseSpecYAML([]byte(yamlSpec))
	if err != nil {
		t.Fatalf("ParseSpecYAML error: %v", err)
	}

	// Compare by re-marshaling to JSON for structural equality
	jsonA, err := json.Marshal(specFromJSON)
	if err != nil {
		t.Fatal(err)
	}
	jsonB, err := json.Marshal(specFromYAML)
	if err != nil {
		t.Fatal(err)
	}

	var a, b Spec
	if err := json.Unmarshal(jsonA, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(jsonB, &b); err != nil {
		t.Fatal(err)
	}

	if a.Name != b.Name {
		t.Errorf("Name mismatch: %q vs %q", a.Name, b.Name)
	}
	if a.Description != b.Description {
		t.Errorf("Description mismatch: %q vs %q", a.Description, b.Description)
	}
	if a.Completion != b.Completion {
		t.Errorf("Completion mismatch: %q vs %q", a.Completion, b.Completion)
	}
	if len(a.Tasks) != len(b.Tasks) {
		t.Fatalf("Tasks length mismatch: %d vs %d", len(a.Tasks), len(b.Tasks))
	}
	for i := range a.Tasks {
		if a.Tasks[i].ID != b.Tasks[i].ID {
			t.Errorf("Tasks[%d].ID: %q vs %q", i, a.Tasks[i].ID, b.Tasks[i].ID)
		}
		if a.Tasks[i].Description != b.Tasks[i].Description {
			t.Errorf("Tasks[%d].Description: %q vs %q", i, a.Tasks[i].Description, b.Tasks[i].Description)
		}
		if len(a.Tasks[i].DependsOn) != len(b.Tasks[i].DependsOn) {
			t.Errorf("Tasks[%d].DependsOn length: %d vs %d", i, len(a.Tasks[i].DependsOn), len(b.Tasks[i].DependsOn))
		}
	}
}
