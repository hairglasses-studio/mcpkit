//go:build !official_sdk

package discovery_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/hairglasses-studio/mcpkit/discovery"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleNewClient() {
	// Start a local registry mock.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := discovery.SearchResult{
			Servers: []discovery.ServerMetadata{
				{ID: "srv-1", Name: "example-server", Description: "An example MCP server"},
			},
			Total: 1, Limit: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	c := discovery.NewClient(discovery.ClientConfig{BaseURL: srv.URL})
	res, err := c.Search(context.Background(), discovery.SearchQuery{Limit: 10})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(res.Servers))
	fmt.Println(res.Servers[0].Name)
	// Output:
	// 1
	// example-server
}

func ExampleNewPublisher() {
	// Start a local registry mock that echoes the posted body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var meta discovery.ServerMetadata
		json.NewDecoder(r.Body).Decode(&meta)
		meta.ID = "registered-id"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	p, err := discovery.NewPublisher(discovery.PublisherConfig{
		BaseURL: srv.URL,
		Token:   "my-token",
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	meta := discovery.ServerMetadata{
		Name:        "my-server",
		Description: "A production MCP server",
		Version:     "1.0.0",
	}
	result, err := p.Register(context.Background(), meta)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.ID)
	fmt.Println(result.Name)
	// Output:
	// registered-id
	// my-server
}

func ExampleMetadataFromRegistry() {
	// Register some tools.
	type echoModule struct{}
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&exampleModule{
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "search", Description: "Search for items"}},
			{Tool: registry.Tool{Name: "fetch", Description: "Fetch a document"}},
		},
	})

	transports := []discovery.TransportInfo{
		{Type: "streamable-http", URL: "https://example.com/mcp"},
	}
	meta := discovery.MetadataFromRegistry("my-server", "An example server", reg, transports)

	fmt.Println(meta.Name)
	fmt.Println(len(meta.Tools))
	fmt.Println(meta.Tools[0].Name)
	fmt.Println(meta.Tools[1].Name)
	// Output:
	// my-server
	// 2
	// fetch
	// search
}

func ExampleMetadataFromConfig() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&exampleModule{
		tools: []registry.ToolDefinition{
			{Tool: registry.Tool{Name: "ping", Description: "Ping the server"}},
		},
	})

	meta := discovery.MetadataFromConfig(discovery.MetadataConfig{
		Name:         "config-server",
		Description:  "Server built from config",
		Version:      "2.0.0",
		Organization: "Acme Corp",
		Tags:         []string{"example", "demo"},
		Tools:        reg,
		Transports:   []discovery.TransportInfo{{Type: "stdio"}},
	})

	fmt.Println(meta.Name)
	fmt.Println(meta.Version)
	fmt.Println(meta.Organization)
	fmt.Println(len(meta.Tags))
	fmt.Println(len(meta.Tools))
	// Output:
	// config-server
	// 2.0.0
	// Acme Corp
	// 2
	// 1
}

func ExampleStaticServerCardHandler() {
	meta := discovery.ServerMetadata{
		Name:        "static-server",
		Description: "A static server card example",
		Version:     "1.0.0",
	}

	handler := discovery.StaticServerCardHandler(meta)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Header().Get("Content-Type"))
	// Output:
	// 200
	// application/json
}

// exampleModule is a minimal ToolModule for use in examples.
type exampleModule struct {
	tools []registry.ToolDefinition
}

func (m *exampleModule) Name() string                      { return "example" }
func (m *exampleModule) Description() string               { return "example module" }
func (m *exampleModule) Tools() []registry.ToolDefinition { return m.tools }
