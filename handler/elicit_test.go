//go:build !official_sdk

package handler

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestElicitForm(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	params := ElicitForm("Enter your name", schema)

	if params.Mode != mcp.ElicitationModeForm {
		t.Errorf("mode = %q, want form", params.Mode)
	}
	if params.Message != "Enter your name" {
		t.Errorf("message = %q", params.Message)
	}
	if params.RequestedSchema == nil {
		t.Error("schema should not be nil")
	}
	if err := params.Validate(); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestElicitURL(t *testing.T) {
	params := ElicitURL("Please authenticate", "elicit-123", "https://auth.example.com/login")

	if params.Mode != mcp.ElicitationModeURL {
		t.Errorf("mode = %q, want url", params.Mode)
	}
	if params.ElicitationID != "elicit-123" {
		t.Errorf("elicitationID = %q", params.ElicitationID)
	}
	if params.URL != "https://auth.example.com/login" {
		t.Errorf("url = %q", params.URL)
	}
	if err := params.Validate(); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestElicitFormSchema(t *testing.T) {
	schema := ElicitFormSchema(
		FormField{Name: "name", Type: "string", Description: "Your name", Required: true},
		FormField{Name: "age", Type: "number", Description: "Your age"},
		FormField{Name: "agree", Type: "boolean", Default: false},
		FormField{Name: "role", Type: "string", Enum: []string{"admin", "user"}},
	)

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not a map")
	}
	if len(props) != 4 {
		t.Errorf("properties count = %d, want 4", len(props))
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required is not a string slice")
	}
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("required = %v, want [name]", required)
	}

	nameProp := props["name"].(map[string]any)
	if nameProp["type"] != "string" {
		t.Errorf("name type = %v", nameProp["type"])
	}

	roleProp := props["role"].(map[string]any)
	if roleProp["enum"] == nil {
		t.Error("role should have enum")
	}
}
