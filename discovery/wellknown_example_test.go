//go:build !official_sdk

package discovery_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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

func ExampleWriteFile() {
	dir, _ := os.MkdirTemp("", "mcp-example-*")
	defer os.RemoveAll(dir)

	cfg := discovery.MetadataConfig{
		Name:       "my-mcp-server",
		Version:    "1.0.0",
		License:    "MIT",
		Homepage:   "https://github.com/example/my-mcp-server",
		Categories: []string{"developer-tools"},
		Install:    &discovery.InstallInfo{Go: "go install github.com/example/my-mcp-server@latest"},
	}

	meta := discovery.MetadataFromConfig(cfg)
	card := discovery.ServerCard{ServerMetadata: meta}

	dest := filepath.Join(dir, ".well-known", "mcp.json")
	if err := discovery.WriteFile(dest, card); err != nil {
		fmt.Println("error:", err)
		return
	}

	data, _ := os.ReadFile(dest)
	var out map[string]any
	json.Unmarshal(data, &out)
	fmt.Println(out["name"])
	fmt.Println(out["license"])
	// Output:
	// my-mcp-server
	// MIT
}

func ExampleHandleContractWrite() {
	dir, _ := os.MkdirTemp("", "mcp-contract-*")
	defer os.RemoveAll(dir)

	cfg := discovery.MetadataConfig{
		Name:    "contract-server",
		Version: "1.0.0",
	}

	dest := filepath.Join(dir, ".well-known", "mcp.json")
	err := discovery.HandleContractWrite(dest, cfg)
	if errors.Is(err, discovery.ErrContractWritten) {
		// Normal path: file was written, server should exit.
		fmt.Println("contract written")
	} else if err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// contract written
}
