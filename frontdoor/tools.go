package frontdoor

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// CategoryDiscovery is the category tag applied to every frontdoor tool.
const CategoryDiscovery = "discovery"

// CatalogInput parameters for tool_catalog.
type CatalogInput struct {
	Category string `json:"category,omitempty" jsonschema:"description=Filter by tool category (exact match)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum results to return (0 means no limit)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"description=Pagination offset"`
}

// CatalogEntry is one row of tool_catalog output.
type CatalogEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Write       bool     `json:"write,omitempty"`
}

// CatalogOutput is the response body of tool_catalog.
type CatalogOutput struct {
	Tools []CatalogEntry `json:"tools"`
	Total int            `json:"total"`
}

func (m *Module) catalogTool() registry.ToolDefinition {
	td := handler.TypedHandler(
		m.toolName("tool_catalog"),
		"List registered tools with minimal metadata. Filter by category, paginate via limit and offset.",
		func(_ context.Context, in CatalogInput) (CatalogOutput, error) {
			defs := m.reg.GetAllToolDefinitions()
			filtered := defs[:0:0]
			for _, d := range defs {
				if in.Category != "" && d.Category != in.Category {
					continue
				}
				filtered = append(filtered, d)
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].Tool.Name < filtered[j].Tool.Name
			})

			total := len(filtered)
			start := min(max(in.Offset, 0), total)
			end := total
			if in.Limit > 0 {
				end = min(end, start+in.Limit)
			}

			entries := make([]CatalogEntry, 0, end-start)
			for _, d := range filtered[start:end] {
				entries = append(entries, CatalogEntry{
					Name:        d.Tool.Name,
					Description: d.Tool.Description,
					Category:    d.Category,
					Tags:        d.Tags,
					Deprecated:  d.Deprecated,
					Write:       d.IsWrite,
				})
			}
			return CatalogOutput{Tools: entries, Total: total}, nil
		},
	)
	td.Category = CategoryDiscovery
	td.Tags = []string{"catalog", "discovery", "frontdoor"}
	return td
}

// SearchInput parameters for tool_search.
type SearchInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query (multi-word supported)"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum hits to return (default 25)"`
}

// SearchHit is one row of tool_search output.
type SearchHit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	Score       int    `json:"score"`
	MatchType   string `json:"match_type"`
}

// SearchOutput is the response body of tool_search.
type SearchOutput struct {
	Hits  []SearchHit `json:"hits"`
	Total int         `json:"total"`
	Query string      `json:"query"`
}

func (m *Module) searchTool() registry.ToolDefinition {
	td := handler.TypedHandler(
		m.toolName("tool_search"),
		"Fuzzy search registered tools by name, tags, category, and description.",
		func(_ context.Context, in SearchInput) (SearchOutput, error) {
			if in.Query == "" {
				return SearchOutput{}, fmt.Errorf("query is required")
			}
			results := m.reg.SearchTools(in.Query)

			limit := in.Limit
			if limit <= 0 {
				limit = 25
			}
			limit = min(limit, len(results))

			hits := make([]SearchHit, 0, limit)
			for _, r := range results[:limit] {
				hits = append(hits, SearchHit{
					Name:        r.Tool.Tool.Name,
					Description: r.Tool.Tool.Description,
					Category:    r.Tool.Category,
					Score:       r.Score,
					MatchType:   r.MatchType,
				})
			}
			return SearchOutput{Hits: hits, Total: len(results), Query: in.Query}, nil
		},
	)
	td.Category = CategoryDiscovery
	td.Tags = []string{"search", "discovery", "frontdoor"}
	return td
}

// SchemaInput parameters for tool_schema.
type SchemaInput struct {
	Name string `json:"name" jsonschema:"required,description=Exact tool name"`
}

// SchemaOutput is the response body of tool_schema.
type SchemaOutput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	InputSchema  any      `json:"input_schema"`
	OutputSchema any      `json:"output_schema,omitempty"`
}

func (m *Module) schemaTool() registry.ToolDefinition {
	td := handler.TypedHandler(
		m.toolName("tool_schema"),
		"Return the input and output schema plus metadata for a tool by exact name.",
		func(_ context.Context, in SchemaInput) (SchemaOutput, error) {
			if in.Name == "" {
				return SchemaOutput{}, fmt.Errorf("name is required")
			}
			tdef, ok := m.reg.GetTool(in.Name)
			if !ok {
				return SchemaOutput{}, fmt.Errorf("tool not found: %s", in.Name)
			}
			return SchemaOutput{
				Name:         tdef.Tool.Name,
				Description:  tdef.Tool.Description,
				Category:     tdef.Category,
				Tags:         tdef.Tags,
				InputSchema:  tdef.Tool.InputSchema,
				OutputSchema: tdef.OutputSchema,
			}, nil
		},
	)
	td.Category = CategoryDiscovery
	td.Tags = []string{"schema", "discovery", "frontdoor"}
	return td
}

// HealthInput takes no parameters.
type HealthInput struct{}

// HealthOutput is the response body of server_health.
type HealthOutput struct {
	Status     string         `json:"status"`
	Uptime     string         `json:"uptime,omitempty"`
	ToolCount  int            `json:"tool_count"`
	ModuleCnt  int            `json:"module_count"`
	Categories map[string]int `json:"categories,omitempty"`
	Timestamp  string         `json:"timestamp"`
}

func (m *Module) healthTool() registry.ToolDefinition {
	td := handler.TypedHandler(
		m.toolName("server_health"),
		"Report server lifecycle status, uptime, and tool inventory counts.",
		func(_ context.Context, _ HealthInput) (HealthOutput, error) {
			out := HealthOutput{
				Status:    "ok",
				ToolCount: m.reg.ToolCount(),
				ModuleCnt: m.reg.ModuleCount(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			stats := m.reg.GetToolStats()
			if len(stats.ByCategory) > 0 {
				out.Categories = stats.ByCategory
			}
			if m.checker != nil {
				out.Status = m.checker.Status()
				out.Uptime = m.checker.Check().Uptime
			}
			return out, nil
		},
	)
	td.Category = CategoryDiscovery
	td.Tags = []string{"health", "discovery", "frontdoor"}
	return td
}
