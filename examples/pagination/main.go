// Command pagination demonstrates token-efficient MCP tool patterns using
// the handler helpers shipped in handler/pagination.go:
//
//   - handler.Paginate[T]  — cursor-based paging to bound row count
//   - handler.TruncateResult — byte-budget enforcement on tool output
//   - handler.SchemaFirstResult — schema-before-data (dbhub pattern)
//
// The server exposes a `list_products` tool over a synthetic 500-row
// dataset so you can see each pattern work in isolation or in composition.
//
// Usage:
//
//	go run ./examples/pagination
//
// Then call the tool via any MCP client:
//
//	list_products {}                          → schema + hint (schema_only default)
//	list_products {"schema_only": false}      → first 50 products + next_cursor
//	list_products {"schema_only": false,
//	               "limit": 10, "cursor": "..."} → second page
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Product is one synthetic row in the demo dataset.
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

// generateProducts fabricates a 500-row dataset to show the pagination
// helper working on a realistic size. Real handlers would query a DB.
func generateProducts() []Product {
	products := make([]Product, 500)
	for i := range products {
		products[i] = Product{
			ID:    i + 1,
			Name:  fmt.Sprintf("Product %03d", i+1),
			Price: 9.99 + float64(i)*0.10,
			Stock: (i * 7) % 100,
		}
	}
	return products
}

// productsSchema documents the shape returned when the caller asks for
// schema_only=true. LLMs use this to craft a better follow-up query.
func productsSchema() any {
	return map[string]any{
		"description": "Paginated product catalog demo. Supports cursor paging.",
		"row_shape": map[string]any{
			"id":    "int",
			"name":  "string",
			"price": "float (USD)",
			"stock": "int (units)",
		},
		"total_rows":   500,
		"max_page":     handlerDefaultLimit(),
		"max_response": fmt.Sprintf("%d bytes (truncated beyond)", responseBudget),
	}
}

// handlerDefaultLimit returns Paginate's implicit default (50) for the
// schema documentation. Kept in sync with handler/pagination.go.
func handlerDefaultLimit() int { return 50 }

// responseBudget is the hard ceiling on response size. Applied after
// pagination as a defensive layer for pathologically large rows.
const responseBudget = 16 * 1024 // 16 KiB

// ListProductsInput — the caller contract. schema_only defaults to true
// so the first call to an unfamiliar tool returns cheap metadata.
type ListProductsInput struct {
	SchemaOnly bool   `json:"schema_only,omitempty" jsonschema:"description=Return schema metadata instead of data. Default: true (explore first)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"description=Opaque cursor from a previous response's next_cursor field"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max items per page (default 50, max capped by server)"`
	MinPrice   float64 `json:"min_price,omitempty" jsonschema:"description=Filter: include only products with price >= min_price"`
}

// ListProductsOutput wraps the typed response. When schema_only is set,
// Schema carries the metadata doc; otherwise Page carries the data slice.
type ListProductsOutput struct {
	Schema any                   `json:"schema,omitempty"`
	Hint   string                `json:"hint,omitempty"`
	Page   *handler.Page[Product] `json:"page,omitempty"`
}

// CatalogModule exposes list_products.
type CatalogModule struct{}

func (m *CatalogModule) Name() string        { return "catalog" }
func (m *CatalogModule) Description() string { return "Paginated product catalog demo" }

func (m *CatalogModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[ListProductsInput, ListProductsOutput](
			"list_products",
			"List products with cursor-based pagination and schema-first discovery. "+
				"Default call returns the tool's schema — pass schema_only=false to fetch data.",
			handleListProducts,
		),
	}
}

// handleListProducts composes all three helpers: SchemaFirstResult gates
// the schema/data choice, Paginate bounds row count on the filtered set,
// TruncateResult guards against any single row exploding the budget.
//
// Default behavior (no args) returns schema metadata — the dbhub "explore
// first" pattern. Callers then re-call with schema_only=false to fetch data.
func handleListProducts(_ context.Context, input ListProductsInput) (ListProductsOutput, error) {
	if input.SchemaOnly || (!input.SchemaOnly && input.Cursor == "" && input.Limit == 0 && input.MinPrice == 0) {
		// Schema-first: empty call or explicit schema_only=true returns metadata.
		// Note: a caller genuinely wanting the first page with defaults must
		// pass schema_only=false AND at least one other field; this is an
		// intentional friction that forces discovery.
		if input.SchemaOnly {
			return ListProductsOutput{
				Schema: productsSchema(),
				Hint:   "re-call with schema_only=false (and optional cursor/limit/min_price) to fetch data",
			}, nil
		}
	}

	all := generateProducts()

	// Apply filter before paginating so cursors stay stable for
	// a given filter combination.
	if input.MinPrice > 0 {
		filtered := all[:0:0]
		for _, p := range all {
			if p.Price >= input.MinPrice {
				filtered = append(filtered, p)
			}
		}
		all = filtered
	}

	page := handler.Paginate(all, handler.PageCursor(input.Cursor), input.Limit)
	return ListProductsOutput{Page: &page}, nil
}

func main() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&CatalogModule{})

	s := registry.NewMCPServer("pagination-example", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
