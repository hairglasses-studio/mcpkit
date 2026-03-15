package rdcycle

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestModule_ImplementsToolModule(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	var _ registry.ToolModule = m // compile-time check
}

func TestModule_Name(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	if m.Name() != "rdcycle" {
		t.Errorf("Name: want %q, got %q", "rdcycle", m.Name())
	}
}

func TestModule_Description(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	if m.Description() == "" {
		t.Error("Description: expected non-empty")
	}
}

func TestModule_Tools_Count(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	tools := m.Tools()
	if len(tools) != 9 {
		t.Errorf("Tools len: want 9, got %d", len(tools))
	}
}

func TestModule_Tools_Names(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	tools := m.Tools()

	want := map[string]bool{
		"rdcycle_scan":      false,
		"rdcycle_plan":      false,
		"rdcycle_verify":    false,
		"rdcycle_artifacts": false,
		"rdcycle_commit":    false,
		"rdcycle_report":    false,
		"rdcycle_schedule":  false,
		"rdcycle_notes":     false,
		"rdcycle_improve":   false,
	}
	for _, td := range tools {
		want[td.Tool.Name] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("Tools: missing tool %q", name)
		}
	}
}

func TestModule_Tools_AllHaveCategory(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	for _, td := range m.Tools() {
		if td.Category != "rdcycle" {
			t.Errorf("tool %q: Category want %q, got %q", td.Tool.Name, "rdcycle", td.Category)
		}
	}
}

func TestModule_Tools_AllHaveTimeout(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	for _, td := range m.Tools() {
		if td.Timeout == 0 {
			t.Errorf("tool %q: Timeout should be set", td.Tool.Name)
		}
	}
}

func TestModule_Tools_VerifyHasLongTimeout(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	for _, td := range m.Tools() {
		if td.Tool.Name == "rdcycle_verify" {
			if td.Timeout.Minutes() < 5 {
				t.Errorf("rdcycle_verify Timeout: want >= 5m, got %v", td.Timeout)
			}
		}
	}
}

func TestModule_Tools_AllHaveHandlers(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})
	for _, td := range m.Tools() {
		if td.Handler == nil {
			t.Errorf("tool %q: Handler must not be nil", td.Tool.Name)
		}
	}
}

func TestModule_RegisterWithRegistry(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	m := NewModule(CycleConfig{})
	reg.RegisterModule(m)

	if reg.ToolCount() != 9 {
		t.Errorf("ToolCount: want 9, got %d", reg.ToolCount())
	}
	if reg.ModuleCount() != 1 {
		t.Errorf("ModuleCount: want 1, got %d", reg.ModuleCount())
	}
}

func TestHandleArtifacts_Empty(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleArtifacts(context.Background(), ArtifactsInput{})
	if err != nil {
		t.Fatalf("handleArtifacts: unexpected error: %v", err)
	}
	if out.Count != 0 {
		t.Errorf("Count: want 0, got %d", out.Count)
	}
	if out.Artifacts == nil {
		t.Error("Artifacts: expected non-nil slice")
	}
}

func TestHandleArtifacts_FilterByType(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	// Store a scan artifact directly.
	_ = m.store.Save(Artifact{ID: "s1", Type: "scan"})
	_ = m.store.Save(Artifact{ID: "p1", Type: "plan"})

	out, err := m.handleArtifacts(context.Background(), ArtifactsInput{Type: "scan"})
	if err != nil {
		t.Fatalf("handleArtifacts: unexpected error: %v", err)
	}
	if out.Count != 1 {
		t.Errorf("Count: want 1, got %d", out.Count)
	}
}

func TestHandleArtifacts_AllTypes(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	_ = m.store.Save(Artifact{ID: "s1", Type: "scan"})
	_ = m.store.Save(Artifact{ID: "p1", Type: "plan"})
	_ = m.store.Save(Artifact{ID: "v1", Type: "verify"})

	out, err := m.handleArtifacts(context.Background(), ArtifactsInput{})
	if err != nil {
		t.Fatalf("handleArtifacts: unexpected error: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("Count: want 3, got %d", out.Count)
	}
}
