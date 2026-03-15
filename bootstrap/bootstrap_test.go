//go:build !official_sdk

package bootstrap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/extensions"
	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// --- test helpers ---

type testToolModule struct{}

func (m *testToolModule) Name() string        { return "test" }
func (m *testToolModule) Description() string { return "test module" }
func (m *testToolModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool:     registry.Tool{Name: "test_tool", Description: "A test tool"},
			Category: "testing",
			Tags:     []string{"test"},
			IsWrite:  true,
		},
	}
}

type testResourceModule struct{}

func (m *testResourceModule) Name() string        { return "test-resources" }
func (m *testResourceModule) Description() string { return "test resource module" }
func (m *testResourceModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.Resource{URI: "file:///test", Name: "test", Description: "test resource"},
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return nil, nil
			},
		},
	}
}
func (m *testResourceModule) Templates() []resources.TemplateDefinition { return nil }

type testPromptModule struct{}

func (m *testPromptModule) Name() string        { return "test-prompts" }
func (m *testPromptModule) Description() string { return "test prompt module" }
func (m *testPromptModule) Prompts() []prompts.PromptDefinition {
	return []prompts.PromptDefinition{
		{
			Prompt: mcp.Prompt{
				Name:        "test_prompt",
				Description: "test prompt",
				Arguments:   []mcp.PromptArgument{{Name: "arg1"}},
			},
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return nil, nil
			},
		},
	}
}

// --- tests ---

func TestGenerateReportToolsOnly(t *testing.T) {
	t.Parallel()

	tr := registry.NewToolRegistry()
	tr.RegisterModule(&testToolModule{})

	report := GenerateReport(Config{
		ServerName: "tools-server",
		Tools:      tr,
	})

	if report.ServerName != "tools-server" {
		t.Errorf("ServerName = %q, want %q", report.ServerName, "tools-server")
	}
	if len(report.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(report.Tools))
	}
	tool := report.Tools[0]
	if tool.Name != "test_tool" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "test_tool")
	}
	if tool.Description != "A test tool" {
		t.Errorf("tool.Description = %q, want %q", tool.Description, "A test tool")
	}
	if tool.Category != "testing" {
		t.Errorf("tool.Category = %q, want %q", tool.Category, "testing")
	}
	if !tool.IsWrite {
		t.Error("tool.IsWrite should be true")
	}
	if len(report.Resources) != 0 {
		t.Errorf("expected no resources, got %d", len(report.Resources))
	}
	if len(report.Prompts) != 0 {
		t.Errorf("expected no prompts, got %d", len(report.Prompts))
	}
	if len(report.Extensions) != 0 {
		t.Errorf("expected no extensions, got %d", len(report.Extensions))
	}
}

func TestGenerateReportAllRegistries(t *testing.T) {
	t.Parallel()

	tr := registry.NewToolRegistry()
	tr.RegisterModule(&testToolModule{})

	rr := resources.NewResourceRegistry()
	rr.RegisterModule(&testResourceModule{})

	pr := prompts.NewPromptRegistry()
	pr.RegisterModule(&testPromptModule{})

	er := extensions.NewExtensionRegistry()
	_ = er.Register(extensions.Extension{Name: "mcpkit:health", Version: "1.0.0"})
	er.Negotiate([]string{"mcpkit:health"})

	report := GenerateReport(Config{
		ServerName: "full-server",
		Tools:      tr,
		Resources:  rr,
		Prompts:    pr,
		Extensions: er,
		Metadata:   map[string]string{"env": "test"},
	})

	if len(report.Tools) != 1 {
		t.Errorf("len(Tools) = %d, want 1", len(report.Tools))
	}
	if len(report.Resources) != 1 {
		t.Fatalf("len(Resources) = %d, want 1", len(report.Resources))
	}
	res := report.Resources[0]
	if res.URI != "file:///test" {
		t.Errorf("resource.URI = %q, want %q", res.URI, "file:///test")
	}
	if res.Name != "test" {
		t.Errorf("resource.Name = %q, want %q", res.Name, "test")
	}
	if res.Description != "test resource" {
		t.Errorf("resource.Description = %q, want %q", res.Description, "test resource")
	}

	if len(report.Prompts) != 1 {
		t.Fatalf("len(Prompts) = %d, want 1", len(report.Prompts))
	}
	p := report.Prompts[0]
	if p.Name != "test_prompt" {
		t.Errorf("prompt.Name = %q, want %q", p.Name, "test_prompt")
	}
	if len(p.Arguments) != 1 || p.Arguments[0] != "arg1" {
		t.Errorf("prompt.Arguments = %v, want [arg1]", p.Arguments)
	}

	if len(report.Extensions) != 1 || report.Extensions[0] != "mcpkit:health" {
		t.Errorf("Extensions = %v, want [mcpkit:health]", report.Extensions)
	}

	if report.Metadata["env"] != "test" {
		t.Errorf("Metadata[env] = %q, want %q", report.Metadata["env"], "test")
	}
}

func TestFormatText(t *testing.T) {
	t.Parallel()

	tr := registry.NewToolRegistry()
	tr.RegisterModule(&testToolModule{})

	rr := resources.NewResourceRegistry()
	rr.RegisterModule(&testResourceModule{})

	pr := prompts.NewPromptRegistry()
	pr.RegisterModule(&testPromptModule{})

	report := GenerateReport(Config{
		ServerName: "text-server",
		Tools:      tr,
		Resources:  rr,
		Prompts:    pr,
		Metadata:   map[string]string{"version": "1.0"},
	})

	text := report.FormatText()

	checks := []string{
		"Server: text-server",
		"Tools (1):",
		"test_tool",
		"[write]",
		"Resources (1):",
		"file:///test",
		"Prompts (1):",
		"test_prompt(arg1)",
		"Metadata:",
		"version: 1.0",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("FormatText missing %q\nfull output:\n%s", want, text)
		}
	}
}

func TestFormatJSON(t *testing.T) {
	t.Parallel()

	tr := registry.NewToolRegistry()
	tr.RegisterModule(&testToolModule{})

	report := GenerateReport(Config{
		ServerName: "json-server",
		Tools:      tr,
		Metadata:   map[string]string{"k": "v"},
	})

	data, err := report.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var decoded ContextReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ServerName != "json-server" {
		t.Errorf("decoded.ServerName = %q, want %q", decoded.ServerName, "json-server")
	}
	if len(decoded.Tools) != 1 {
		t.Fatalf("decoded len(Tools) = %d, want 1", len(decoded.Tools))
	}
	if decoded.Tools[0].Name != "test_tool" {
		t.Errorf("decoded tool name = %q, want %q", decoded.Tools[0].Name, "test_tool")
	}
	if decoded.Metadata["k"] != "v" {
		t.Errorf("decoded Metadata[k] = %q, want %q", decoded.Metadata["k"], "v")
	}
	if decoded.GeneratedAt.IsZero() {
		t.Error("decoded GeneratedAt should not be zero")
	}
}

func TestGenerateReportEmptyRegistries(t *testing.T) {
	t.Parallel()

	tr := registry.NewToolRegistry()
	rr := resources.NewResourceRegistry()
	pr := prompts.NewPromptRegistry()

	report := GenerateReport(Config{
		ServerName: "empty-server",
		Tools:      tr,
		Resources:  rr,
		Prompts:    pr,
	})

	if report.ServerName != "empty-server" {
		t.Errorf("ServerName = %q, want %q", report.ServerName, "empty-server")
	}
	if len(report.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(report.Tools))
	}
	if len(report.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(report.Resources))
	}
	if len(report.Prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(report.Prompts))
	}
}

func TestGenerateReportNilRegistries(t *testing.T) {
	t.Parallel()

	// All optional registries nil — should not panic
	report := GenerateReport(Config{
		ServerName: "minimal-server",
	})

	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if report.ServerName != "minimal-server" {
		t.Errorf("ServerName = %q, want %q", report.ServerName, "minimal-server")
	}
	if len(report.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(report.Tools))
	}
	if len(report.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(report.Resources))
	}
	if len(report.Prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(report.Prompts))
	}
	if len(report.Extensions) != 0 {
		t.Errorf("expected 0 extensions, got %d", len(report.Extensions))
	}
}

func TestFormatTextNoWriteFlag(t *testing.T) {
	t.Parallel()

	// A read-only tool should not show [write]
	tr := registry.NewToolRegistry()
	tr.RegisterModule(&struct{ testToolModule }{})

	// Build report directly with a read-only tool summary
	report := &ContextReport{
		ServerName: "ro-server",
		Tools: []ToolSummary{
			{Name: "read_tool", Description: "reads stuff", IsWrite: false},
		},
	}

	text := report.FormatText()
	if strings.Contains(text, "[write]") {
		t.Errorf("FormatText should not contain [write] for a read-only tool, got:\n%s", text)
	}
	if !strings.Contains(text, "read_tool") {
		t.Errorf("FormatText missing tool name, got:\n%s", text)
	}
}
