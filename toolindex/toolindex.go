//go:build !official_sdk

package toolindex

import (
	"context"
	"sort"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// catalogInput is the input for the {prefix}_tool_catalog tool.
type catalogInput struct {
	Category string `json:"category,omitempty" jsonschema:"description=Optional category filter. Omit to list all categories."`
}

// catalogEntry describes a single tool in the catalog.
type catalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsWrite     bool   `json:"is_write"`
}

// catalogGroup is a category group in the catalog output.
type catalogGroup struct {
	Category  string         `json:"category"`
	ToolCount int            `json:"tool_count"`
	Tools     []catalogEntry `json:"tools"`
}

// catalogOutput is the output of the {prefix}_tool_catalog tool.
type catalogOutput struct {
	Groups     []catalogGroup `json:"groups"`
	TotalTools int            `json:"total_tools"`
}

// searchInput is the input for the {prefix}_tool_search tool.
type searchInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query to match against tool names and descriptions"`
}

// searchResult describes a single search hit.
type searchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsWrite     bool   `json:"is_write"`
}

// searchOutput is the output of the {prefix}_tool_search tool.
type searchOutput struct {
	Results []searchResult `json:"results"`
	Total   int            `json:"total"`
}

// toolIndexModule implements registry.ToolModule and provides tool catalog/search.
type toolIndexModule struct {
	prefix string
	reg    *registry.ToolRegistry
}

// NewToolIndexModule creates a ToolModule that exposes tool catalog/search
// tools for the given registry. Any MCP server can call:
//
//	reg.RegisterModule(toolindex.NewToolIndexModule("myserver", reg))
//
// to get {prefix}_tool_catalog and {prefix}_tool_search tools for free.
func NewToolIndexModule(prefix string, reg *registry.ToolRegistry) registry.ToolModule {
	return &toolIndexModule{prefix: prefix, reg: reg}
}

func (m *toolIndexModule) Name() string        { return m.prefix + "_tool_index" }
func (m *toolIndexModule) Description() string  { return "Tool catalog and search for " + m.prefix }
func (m *toolIndexModule) Tools() []registry.ToolDefinition {
	catalog := handler.TypedHandler[catalogInput, catalogOutput](
		m.prefix+"_tool_catalog",
		"List all registered tools grouped by category. Use this to discover available capabilities before invoking a tool.",
		func(_ context.Context, input catalogInput) (catalogOutput, error) {
			return m.buildCatalog(input.Category), nil
		},
	)
	catalog.Category = "discovery"
	catalog.SearchTerms = []string{"tool catalog", "tool list", "categories", "browse tools"}

	search := handler.TypedHandler[searchInput, searchOutput](
		m.prefix+"_tool_search",
		"Search registered tools by keyword. Matches against tool names and descriptions.",
		func(_ context.Context, input searchInput) (searchOutput, error) {
			return m.searchTools(input.Query), nil
		},
	)
	search.Category = "discovery"
	search.SearchTerms = []string{"find tool", "which tool", "tool discovery", "search"}

	return []registry.ToolDefinition{catalog, search}
}

func (m *toolIndexModule) buildCatalog(categoryFilter string) catalogOutput {
	allTools := m.reg.GetAllToolDefinitions()

	// Group by category.
	grouped := make(map[string][]catalogEntry)
	for _, td := range allTools {
		cat := td.Category
		if cat == "" {
			cat = "general"
		}
		if categoryFilter != "" && cat != categoryFilter {
			continue
		}
		grouped[cat] = append(grouped[cat], catalogEntry{
			Name:        td.Tool.Name,
			Description: td.Tool.Description,
			IsWrite:     td.IsWrite,
		})
	}

	// Sort categories and tools within each category.
	categories := make([]string, 0, len(grouped))
	for cat := range grouped {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	var out catalogOutput
	for _, cat := range categories {
		tools := grouped[cat]
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
		out.Groups = append(out.Groups, catalogGroup{
			Category:  cat,
			ToolCount: len(tools),
			Tools:     tools,
		})
		out.TotalTools += len(tools)
	}
	return out
}

func (m *toolIndexModule) searchTools(query string) searchOutput {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return searchOutput{}
	}

	allTools := m.reg.GetAllToolDefinitions()
	var results []searchResult
	for _, td := range allTools {
		name := strings.ToLower(td.Tool.Name)
		desc := strings.ToLower(td.Tool.Description)
		if strings.Contains(name, query) || strings.Contains(desc, query) {
			results = append(results, searchResult{
				Name:        td.Tool.Name,
				Description: td.Tool.Description,
				Category:    td.Category,
				IsWrite:     td.IsWrite,
			})
		}
	}

	// Sort results by name for deterministic output.
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	return searchOutput{
		Results: results,
		Total:   len(results),
	}
}
