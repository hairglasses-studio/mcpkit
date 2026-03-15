//go:build !official_sdk

package discovery_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/hairglasses-studio/mcpkit/discovery"
)

func ExampleServerCardHandler() {
	handler := discovery.ServerCardHandler(discovery.MetadataConfig{
		Name:        "my-server",
		Description: "Example MCP server",
		Version:     "1.0.0",
	})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Header().Get("Content-Type"))
	// Output:
	// 200
	// application/json
}
