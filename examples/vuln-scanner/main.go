//go:build !official_sdk

// Command vuln-scanner demonstrates a Go module security scanning MCP server
// built with mcpkit. It exposes two tools:
//
//   - vuln_scan: run govulncheck on a Go module directory and return structured
//     vulnerability results with severity classification
//   - vuln_osv_query: query the OSV API (api.osv.dev) for known vulnerabilities
//     affecting a specific Go module version
//
// # Prerequisites
//
// govulncheck must be installed for vuln_scan to work:
//
//	go install golang.org/x/vuln/cmd/govulncheck@latest
//
// # Usage
//
//	go run ./examples/vuln-scanner
//
// # Scan a specific directory
//
//	# Via an MCP client, call vuln_scan with {"dir": "/path/to/module"}
//
// # Query the OSV API
//
//	# Via an MCP client, call vuln_osv_query with {"module":"golang.org/x/net","version":"v0.50.0"}
package main

import (
	"log"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/vuln"
)

func main() {
	reg := registry.NewToolRegistry()

	// Register the vuln module — provides vuln_scan and vuln_osv_query tools.
	reg.RegisterModule(vuln.NewModule())

	s := registry.NewMCPServer("vuln-scanner", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
