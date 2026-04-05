//go:build !official_sdk

package openapi

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestOperationToTool_OperationId(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	byName := make(map[string]registry.ToolDefinition)
	for _, d := range defs {
		byName[d.Tool.Name] = d
	}

	// listPets should have operationId as name.
	if td, ok := byName["listPets"]; !ok {
		t.Error("missing tool: listPets")
	} else {
		if td.Tool.Description != "List all pets" {
			t.Errorf("listPets description = %q, want %q", td.Tool.Description, "List all pets")
		}
		if td.IsWrite {
			t.Error("listPets should not be a write tool (GET)")
		}
		if td.Category != "pets" {
			t.Errorf("listPets category = %q, want %q", td.Category, "pets")
		}
	}
}

func TestOperationToTool_WriteDetection(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	byName := make(map[string]registry.ToolDefinition)
	for _, d := range defs {
		byName[d.Tool.Name] = d
	}

	tests := []struct {
		name    string
		isWrite bool
	}{
		{"listPets", false},
		{"getPet", false},
		{"createPet", true},
		{"deletePet", true},
	}
	for _, tt := range tests {
		td, ok := byName[tt.name]
		if !ok {
			t.Errorf("missing tool: %s", tt.name)
			continue
		}
		if td.IsWrite != tt.isWrite {
			t.Errorf("%s: IsWrite = %v, want %v", tt.name, td.IsWrite, tt.isWrite)
		}
	}
}

func TestOperationToTool_PathParams_Required(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	byName := make(map[string]registry.ToolDefinition)
	for _, d := range defs {
		byName[d.Tool.Name] = d
	}

	td, ok := byName["getPet"]
	if !ok {
		t.Fatal("missing tool: getPet")
	}

	// petId should be in the schema properties.
	props := td.Tool.InputSchema.Properties
	if _, ok := props["petId"]; !ok {
		t.Error("getPet should have petId property")
	}

	// petId should be required.
	found := false
	for _, r := range td.Tool.InputSchema.Required {
		if r == "petId" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("getPet: petId should be required; required = %v", td.Tool.InputSchema.Required)
	}
}

func TestOperationToTool_QueryParams(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	byName := make(map[string]registry.ToolDefinition)
	for _, d := range defs {
		byName[d.Tool.Name] = d
	}

	td, ok := byName["listPets"]
	if !ok {
		t.Fatal("missing tool: listPets")
	}

	props := td.Tool.InputSchema.Properties
	if _, ok := props["limit"]; !ok {
		t.Error("listPets should have limit property")
	}
	if _, ok := props["status"]; !ok {
		t.Error("listPets should have status property")
	}

	// status should have enum in its property.
	statusProp, ok := props["status"].(map[string]any)
	if !ok {
		t.Fatal("status property should be map[string]any")
	}
	if _, hasEnum := statusProp["enum"]; !hasEnum {
		t.Error("status property should have enum")
	}
}

func TestOperationToTool_RequestBody(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	byName := make(map[string]registry.ToolDefinition)
	for _, d := range defs {
		byName[d.Tool.Name] = d
	}

	td, ok := byName["createPet"]
	if !ok {
		t.Fatal("missing tool: createPet")
	}

	props := td.Tool.InputSchema.Properties
	bodyProp, ok := props["body"]
	if !ok {
		t.Fatal("createPet should have body property")
	}

	bodyMap, ok := bodyProp.(map[string]any)
	if !ok {
		t.Fatal("body property should be map[string]any")
	}
	if bodyMap["type"] != "object" {
		t.Errorf("body type = %v, want object", bodyMap["type"])
	}

	// body should be required since the request body is required.
	found := false
	for _, r := range td.Tool.InputSchema.Required {
		if r == "body" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("createPet: body should be required; required = %v", td.Tool.InputSchema.Required)
	}
}

func TestOperationToTool_FallbackName(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	defs := b.buildToolDefinitions()
	// The vaccinations endpoint has no operationId, so it should fall back
	// to path_method style.
	found := false
	for _, d := range defs {
		if d.Tool.Name == "get_pets_petId_vaccinations" {
			found = true
			if d.Category != "vaccinations" {
				t.Errorf("vaccinations category = %q, want %q", d.Category, "vaccinations")
			}
			break
		}
	}
	if !found {
		var names []string
		for _, d := range defs {
			names = append(names, d.Tool.Name)
		}
		t.Errorf("missing fallback tool name 'get_pets_petId_vaccinations'; tools: %v", names)
	}
}

func TestPathMethodName(t *testing.T) {
	tests := []struct {
		path   string
		method string
		want   string
	}{
		{"/pets", "GET", "get_pets"},
		{"/pets/{petId}", "GET", "get_pets_petId"},
		{"/pets/{petId}/vaccinations", "POST", "post_pets_petId_vaccinations"},
	}
	for _, tt := range tests {
		got := pathMethodName(tt.path, tt.method)
		if got != tt.want {
			t.Errorf("pathMethodName(%q, %q) = %q, want %q", tt.path, tt.method, got, tt.want)
		}
	}
}
