//go:build !official_sdk

package tck

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// conformanceModule is a known-good module that passes all TCK checks.
type conformanceModule struct{}

func (m *conformanceModule) Name() string        { return "conformance" }
func (m *conformanceModule) Description() string { return "TCK conformance test module" }
func (m *conformanceModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "tck_echo",
				Description: "Echoes the input message",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"message": map[string]any{"type": "string", "description": "Message to echo"},
					},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				msg := handler.GetStringParam(req, "message")
				return handler.TextResult(msg), nil
			},
			Category: "test",
		},
		{
			Tool: mcp.Tool{
				Name:        "tck_add",
				Description: "Adds two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"a": map[string]any{"type": "number", "description": "First number"},
						"b": map[string]any{"type": "number", "description": "Second number"},
					},
					Required: []string{"a", "b"},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				a := handler.GetFloatParam(req, "a", 0)
				b := handler.GetFloatParam(req, "b", 0)
				return handler.TextResult(fmt.Sprintf("%g", a+b)), nil
			},
			Category: "test",
		},
		{
			Tool: mcp.Tool{
				Name:        "tck_error",
				Description: "Always returns a coded error",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]any{},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return handler.CodedErrorResult(handler.ErrNotFound, fmt.Errorf("resource not found")), nil
			},
			Category: "test",
		},
	}
}

// newConformanceRegistry creates a registry with the conformance module.
func newConformanceRegistry() *registry.ToolRegistry {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&conformanceModule{})
	return reg
}

// TestTCKRunAll runs the entire TCK suite against a known-good server.
func TestTCKRunAll(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)
	suite.Run(t)

	// Verify all checks passed.
	for _, r := range suite.Results {
		if !r.Passed {
			t.Errorf("unexpected failure: %s/%s: %s", r.Category, r.Name, r.Message)
		}
	}
}

// TestTCKRunCategory runs only the tools category.
func TestTCKRunCategory(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)
	suite.RunCategory(t, "tools")

	toolChecks := 0
	for _, r := range suite.Results {
		if r.Category == "tools" {
			toolChecks++
		}
	}
	if toolChecks == 0 {
		t.Error("RunCategory('tools') ran no tool checks")
	}
}

// TestTCKRunLifecycle runs only the lifecycle category.
func TestTCKRunLifecycle(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)
	suite.RunCategory(t, "lifecycle")

	lifecycleChecks := 0
	for _, r := range suite.Results {
		if r.Category == "lifecycle" {
			lifecycleChecks++
		}
	}
	if lifecycleChecks == 0 {
		t.Error("RunCategory('lifecycle') ran no lifecycle checks")
	}
}

// TestTCKSummary verifies the Summary output is populated.
func TestTCKSummary(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)
	suite.Run(t)

	summary := suite.Summary()
	if summary == "" {
		t.Error("Summary() returned empty string")
	}
	if !containsSubstring(summary, "TCK Summary") {
		t.Error("Summary() missing 'TCK Summary' header")
	}
	if !containsSubstring(summary, "passed") {
		t.Error("Summary() missing 'passed' count")
	}
}

// TestTCKEmptyRegistryFails verifies checks correctly fail on an empty registry.
func TestTCKEmptyRegistryFails(t *testing.T) {
	reg := registry.NewToolRegistry()
	_ = NewSuite(reg) // ensure NewSuite works on empty registry

	// Run individual checks directly to avoid sub-test failures.
	result := checkToolsListNotEmpty(reg)
	if result.Passed {
		t.Error("ToolsListNotEmpty should fail on empty registry")
	}

	result = checkModulesRegistered(reg)
	if result.Passed {
		t.Error("ModulesRegistered should fail on empty registry")
	}
}

// TestTCKHandlerContractViolation verifies the handler contract check
// detects when a handler returns (nil, error).
func TestTCKHandlerContractViolation(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&badHandlerModule{})

	result := checkToolHandlerContract(reg)
	if result.Passed {
		t.Error("ToolHandlerContract should fail when handler returns (nil, error)")
	}
}

// TestTCKMissingDescription verifies the description check catches tools
// without descriptions.
func TestTCKMissingDescription(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&noDescModule{})

	result := checkToolsHaveDescriptions(reg)
	if result.Passed {
		t.Error("ToolsHaveDescriptions should fail when tool has no description")
	}
}

// TestTCKUncodedError verifies the error code check catches uncoded errors.
func TestTCKUncodedError(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&uncodedErrorModule{})

	result := checkToolErrorCodes(reg)
	if result.Passed {
		t.Error("ToolErrorCodes should fail when error result has no code prefix")
	}
}

// TestTCKAddCheck verifies custom checks can be added.
func TestTCKAddCheck(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)

	customRan := false
	suite.AddCheck(Check{
		Category: "custom",
		Name:     "AlwaysPass",
		Fn: func(reg *registry.ToolRegistry) CheckResult {
			customRan = true
			return CheckResult{Passed: true, Message: "custom check passed"}
		},
	})

	suite.Run(t)
	if !customRan {
		t.Error("custom check was not executed")
	}
}

// TestTCKCheckCount verifies the minimum check count is met.
func TestTCKCheckCount(t *testing.T) {
	reg := newConformanceRegistry()
	suite := NewSuite(reg)
	suite.Run(t)

	if len(suite.Results) < 10 {
		t.Errorf("expected at least 10 checks, got %d", len(suite.Results))
	}
}

// --- Bad modules for negative testing ---

// badHandlerModule has a handler that violates the contract by returning (nil, error).
type badHandlerModule struct{}

func (m *badHandlerModule) Name() string        { return "bad-handler" }
func (m *badHandlerModule) Description() string { return "Module with bad handler" }
func (m *badHandlerModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "bad_tool",
				Description: "Violates handler contract",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]any{},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, fmt.Errorf("contract violation")
			},
			Category: "test",
		},
	}
}

// noDescModule has a tool with no description.
type noDescModule struct{}

func (m *noDescModule) Name() string        { return "no-desc" }
func (m *noDescModule) Description() string { return "Module with missing tool description" }
func (m *noDescModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name: "undescribed_tool",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]any{},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return handler.TextResult("ok"), nil
			},
			Category: "test",
		},
	}
}

// uncodedErrorModule has a tool that returns an error without coded format.
type uncodedErrorModule struct{}

func (m *uncodedErrorModule) Name() string        { return "uncoded-error" }
func (m *uncodedErrorModule) Description() string { return "Module with uncoded error" }
func (m *uncodedErrorModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "uncoded_error_tool",
				Description: "Returns uncoded error",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]any{},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Returns error result without "[CODE]" prefix — violates convention.
				return registry.MakeErrorResult("something went wrong"), nil
			},
			Category: "test",
		},
	}
}

// containsSubstring is a test helper to check string containment.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
