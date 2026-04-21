//go:build !official_sdk

package frontdoor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type stubModule struct {
	name  string
	tools []registry.ToolDefinition
}

func (m *stubModule) Name() string                     { return m.name }
func (m *stubModule) Description() string              { return "stub" }
func (m *stubModule) Tools() []registry.ToolDefinition { return m.tools }

func makeTool(name, desc, category string, tags []string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name, Description: desc},
		Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
			return registry.MakeTextResult("ok"), nil
		},
		Category: category,
		Tags:     tags,
	}
}

func seedRegistry(t *testing.T) *registry.ToolRegistry {
	t.Helper()
	r := registry.NewToolRegistry()
	r.RegisterModule(&stubModule{
		name: "seeds",
		tools: []registry.ToolDefinition{
			makeTool("alpha_list", "List alpha items", "alpha", []string{"read"}),
			makeTool("alpha_create", "Create alpha item", "alpha", []string{"write"}),
			makeTool("beta_search", "Search beta items", "beta", []string{"read", "search"}),
		},
	})
	return r
}

func invokeTool(t *testing.T, td registry.ToolDefinition, args map[string]any) *registry.CallToolResult {
	t.Helper()
	req := registry.CallToolRequest{}
	req.Params.Name = td.Tool.Name
	req.Params.Arguments = args
	res, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil {
		t.Fatal("handler returned nil result")
	}
	return res
}

func resultText(t *testing.T, res *registry.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	for _, c := range res.Content {
		if tc, ok := c.(registry.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatal("no text content in result")
	return ""
}

func TestNew_DefaultModule(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := New(reg)
	if m.Name() != ModuleName {
		t.Errorf("Name() = %q, want %q", m.Name(), ModuleName)
	}
	tools := m.Tools()
	if len(tools) != 4 {
		t.Fatalf("Tools() count = %d, want 4", len(tools))
	}
	want := []string{"tool_catalog", "tool_search", "tool_schema", "server_health"}
	for i, td := range tools {
		if td.Tool.Name != want[i] {
			t.Errorf("tool[%d].Name = %q, want %q", i, td.Tool.Name, want[i])
		}
		if td.Category != CategoryDiscovery {
			t.Errorf("tool[%d].Category = %q, want %q", i, td.Category, CategoryDiscovery)
		}
	}
}

func TestNew_WithPrefix(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := New(reg, WithPrefix("myapp_"))
	tools := m.Tools()
	want := []string{"myapp_tool_catalog", "myapp_tool_search", "myapp_tool_schema", "myapp_server_health"}
	for i, td := range tools {
		if td.Tool.Name != want[i] {
			t.Errorf("tool[%d].Name = %q, want %q", i, td.Tool.Name, want[i])
		}
	}
}

func TestCatalog_AllTools(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.catalogTool(), nil)

	var out CatalogOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Total != 3 {
		t.Errorf("Total = %d, want 3", out.Total)
	}
	if len(out.Tools) != 3 {
		t.Fatalf("Tools count = %d, want 3", len(out.Tools))
	}
	if out.Tools[0].Name != "alpha_create" {
		t.Errorf("Tools sorted incorrectly — first = %q", out.Tools[0].Name)
	}
}

func TestCatalog_CategoryFilter(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.catalogTool(), map[string]any{"category": "alpha"})

	var out CatalogOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total != 2 {
		t.Errorf("Total = %d, want 2 (alpha only)", out.Total)
	}
	for _, e := range out.Tools {
		if e.Category != "alpha" {
			t.Errorf("got category %q, want alpha", e.Category)
		}
	}
}

func TestCatalog_Pagination(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)

	// Limit = 2, offset = 1 → expect 2 of 3 tools, starting at index 1 (alpha_list)
	res := invokeTool(t, m.catalogTool(), map[string]any{"limit": 2, "offset": 1})
	var out CatalogOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total != 3 {
		t.Errorf("Total = %d, want 3 (unaffected by pagination)", out.Total)
	}
	if len(out.Tools) != 2 {
		t.Fatalf("Tools count = %d, want 2", len(out.Tools))
	}
	if out.Tools[0].Name != "alpha_list" {
		t.Errorf("first after offset = %q, want alpha_list", out.Tools[0].Name)
	}
}

func TestCatalog_OffsetBeyondTotal(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.catalogTool(), map[string]any{"offset": 999})
	var out CatalogOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Tools) != 0 {
		t.Errorf("Tools count = %d, want 0 for out-of-range offset", len(out.Tools))
	}
	if out.Total != 3 {
		t.Errorf("Total = %d, want 3 (unfiltered)", out.Total)
	}
}

func TestSearch_ByName(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.searchTool(), map[string]any{"query": "alpha"})

	var out SearchOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total < 2 {
		t.Errorf("Total = %d, want at least 2 for 'alpha' query", out.Total)
	}
	if len(out.Hits) == 0 {
		t.Fatal("no hits returned")
	}
	// Score must be positive and match_type must be populated.
	if out.Hits[0].Score <= 0 {
		t.Errorf("first hit score = %d, want positive", out.Hits[0].Score)
	}
	if out.Hits[0].MatchType == "" {
		t.Error("first hit MatchType is empty")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.searchTool(), map[string]any{"query": ""})
	if !res.IsError {
		t.Error("expected IsError=true for empty query")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "query is required") {
		t.Errorf("error text = %q, want 'query is required'", text)
	}
}

func TestSearch_LimitClamped(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.searchTool(), map[string]any{"query": "alpha", "limit": 100})

	var out SearchOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Hits) > out.Total {
		t.Errorf("hits %d exceeds total %d", len(out.Hits), out.Total)
	}
}

func TestSchema_KnownTool(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.schemaTool(), map[string]any{"name": "alpha_list"})

	var out SchemaOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Name != "alpha_list" {
		t.Errorf("Name = %q, want alpha_list", out.Name)
	}
	if out.Category != "alpha" {
		t.Errorf("Category = %q, want alpha", out.Category)
	}
}

func TestSchema_UnknownTool(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.schemaTool(), map[string]any{"name": "nope"})
	if !res.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
	text := resultText(t, res)
	if !strings.Contains(text, "tool not found") {
		t.Errorf("error text = %q, want 'tool not found'", text)
	}
}

func TestSchema_EmptyName(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.schemaTool(), map[string]any{"name": ""})
	if !res.IsError {
		t.Error("expected IsError=true for empty name")
	}
}

func TestHealth_NoChecker(t *testing.T) {
	reg := seedRegistry(t)
	m := New(reg)
	res := invokeTool(t, m.healthTool(), nil)

	var out HealthOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "ok" {
		t.Errorf("Status = %q, want ok", out.Status)
	}
	if out.ToolCount != 3 {
		t.Errorf("ToolCount = %d, want 3", out.ToolCount)
	}
	if out.ModuleCnt != 1 {
		t.Errorf("ModuleCnt = %d, want 1", out.ModuleCnt)
	}
	if out.Timestamp == "" {
		t.Error("Timestamp is empty")
	}
	if out.Categories["alpha"] != 2 {
		t.Errorf("Categories[alpha] = %d, want 2", out.Categories["alpha"])
	}
}

func TestHealth_WithChecker(t *testing.T) {
	reg := seedRegistry(t)
	chk := health.NewChecker()
	chk.SetStatus("draining")
	m := New(reg, WithHealthChecker(chk))
	res := invokeTool(t, m.healthTool(), nil)

	var out HealthOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "draining" {
		t.Errorf("Status = %q, want draining (from checker)", out.Status)
	}
	if out.Uptime == "" {
		t.Error("Uptime is empty with checker attached")
	}
}

func TestModule_RegistersOnRegistry(t *testing.T) {
	reg := seedRegistry(t)
	reg.RegisterModule(New(reg, WithPrefix("foo_")))

	for _, name := range []string{"foo_tool_catalog", "foo_tool_search", "foo_tool_schema", "foo_server_health"} {
		if _, ok := reg.GetTool(name); !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}
