//go:build !official_sdk

package openapi

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestNewBridge_NilRegistry(t *testing.T) {
	_, err := NewBridge("testdata/petstore.json", nil, BridgeConfig{})
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestNewBridge_EmptySpec(t *testing.T) {
	reg := registry.NewToolRegistry()
	_, err := NewBridge("", reg, BridgeConfig{})
	if err == nil {
		t.Fatal("expected error for empty spec path")
	}
}

func TestNewBridge_InvalidSpec(t *testing.T) {
	reg := registry.NewToolRegistry()
	_, err := NewBridge("/nonexistent/spec.json", reg, BridgeConfig{})
	if err == nil {
		t.Fatal("expected error for invalid spec path")
	}
}

func TestNewBridge_LoadPetstore(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if b.spec == nil {
		t.Fatal("spec should not be nil")
	}
	if b.baseURL != "http://localhost:9999" {
		t.Errorf("baseURL = %q, want %q", b.baseURL, "http://localhost:9999")
	}
}

func TestNewBridge_BaseURLOverride(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{
		BaseURL: "http://custom:8080",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if b.baseURL != "http://custom:8080" {
		t.Errorf("baseURL = %q, want %q", b.baseURL, "http://custom:8080")
	}
}

func TestRegisterTools_Count(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	// Petstore has 5 operations: listPets, createPet, getPet, deletePet,
	// and GET /pets/{petId}/vaccinations (no operationId).
	got := reg.ToolCount()
	if got != 5 {
		t.Errorf("ToolCount = %d, want 5; tools: %v", got, reg.ListTools())
	}
}

func TestRegisterTools_ToolNames_OperationId(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	want := map[string]bool{
		"listPets":  true,
		"createPet": true,
		"getPet":    true,
		"deletePet": true,
	}
	for _, name := range reg.ListTools() {
		if want[name] {
			delete(want, name)
		}
	}
	for name := range want {
		t.Errorf("missing tool: %s", name)
	}
}

func TestRegisterTools_ToolNames_PathMethod(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{
		NameStyle: "path_method",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	// All tools should use path_method style.
	tools := reg.ListTools()
	for _, name := range tools {
		// path_method names start with the HTTP method in lowercase.
		if !startsWithMethod(name) {
			t.Errorf("tool %q does not start with HTTP method", name)
		}
	}
}

func startsWithMethod(name string) bool {
	for _, m := range []string{"get_", "post_", "put_", "delete_", "patch_"} {
		if len(name) > len(m) && name[:len(m)] == m {
			return true
		}
	}
	return false
}

func TestRegisterTools_Module(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	modules := reg.ListModules()
	found := false
	for _, m := range modules {
		if m == "openapi" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("module 'openapi' not found; modules: %v", modules)
	}
}

func TestToolCount(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if got := b.ToolCount(); got != 5 {
		t.Errorf("ToolCount = %d, want 5", got)
	}
}
