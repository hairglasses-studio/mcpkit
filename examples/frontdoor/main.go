// Command frontdoor demonstrates the frontdoor discovery-first starter
// package. It stands up a tiny MCP server with two domain tools
// (inventory_search and inventory_add) plus the frontdoor module, which
// exposes tool_catalog, tool_search, tool_schema, and server_health for
// any MCP client that wants to explore the surface.
//
// Usage:
//
//	go run ./examples/frontdoor
//
// Then, from an MCP client, call `tool_catalog`, `tool_search`, or
// `server_health` to discover what the server offers without hardcoding
// any tool names into the client.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hairglasses-studio/mcpkit/frontdoor"
	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/health"
	"github.com/hairglasses-studio/mcpkit/registry"
)

type InventoryItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"required,description=Substring to match against item names"`
}

type SearchOutput struct {
	Items []InventoryItem `json:"items"`
}

type AddInput struct {
	Name     string `json:"name" jsonschema:"required,description=Item name"`
	Quantity int    `json:"quantity" jsonschema:"required,description=Quantity to add (can be negative)"`
}

type AddOutput struct {
	Total int `json:"total"`
}

type InventoryModule struct {
	items map[string]int
}

func (m *InventoryModule) Name() string        { return "inventory" }
func (m *InventoryModule) Description() string { return "Tiny in-memory inventory demo" }

func (m *InventoryModule) Tools() []registry.ToolDefinition {
	search := handler.TypedHandler(
		"inventory_search",
		"Search inventory items by name substring.",
		func(_ context.Context, in SearchInput) (SearchOutput, error) {
			q := strings.ToLower(in.Query)
			var hits []InventoryItem
			for name, qty := range m.items {
				if strings.Contains(strings.ToLower(name), q) {
					hits = append(hits, InventoryItem{Name: name, Quantity: qty})
				}
			}
			return SearchOutput{Items: hits}, nil
		},
	)
	search.Category = "inventory"
	search.Tags = []string{"read", "search"}

	add := handler.TypedHandler(
		"inventory_add",
		"Add or subtract inventory for an item. Negative quantities decrement.",
		func(_ context.Context, in AddInput) (AddOutput, error) {
			if in.Name == "" {
				return AddOutput{}, fmt.Errorf("name is required")
			}
			m.items[in.Name] += in.Quantity
			return AddOutput{Total: m.items[in.Name]}, nil
		},
	)
	add.Category = "inventory"
	add.Tags = []string{"write"}
	add.IsWrite = true

	return []registry.ToolDefinition{search, add}
}

func main() {
	reg := registry.NewToolRegistry()

	inv := &InventoryModule{items: map[string]int{
		"widget":   3,
		"gadget":   7,
		"sprocket": 12,
	}}
	reg.RegisterModule(inv)

	chk := health.NewChecker(
		health.WithToolCount(reg.ToolCount),
	)

	reg.RegisterModule(frontdoor.New(reg,
		frontdoor.WithPrefix("fd_"),
		frontdoor.WithHealthChecker(chk),
	))

	s := registry.NewMCPServer("frontdoor-example", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
