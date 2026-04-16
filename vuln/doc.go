// Package vuln provides Go module security scanning tools for MCP servers.
//
// It wraps govulncheck to scan Go modules for known vulnerabilities and
// optionally enriches results with OSV (Open Source Vulnerabilities) API
// data. Results are returned as structured [ScanResult] values suitable
// for downstream reporting and MCP tool handlers.
//
// # govulncheck integration
//
// [Scanner.Scan] invokes govulncheck -format json on a target directory and
// parses the streaming JSON output into [Vulnerability] records. Each record
// carries the OSV ID, summary, severity estimate, affected module path and
// version, and the fixed version (if available).
//
// # OSV API enrichment
//
// [OSVClient.Query] queries the OSV REST API (https://api.osv.dev/v1/query) by
// module path and version to retrieve the full vulnerability record. This is
// useful when govulncheck is not available or when enriching results with
// additional advisory details like CVE aliases and advisory URLs.
//
// # MCP module
//
// [NewModule] returns a [registry.ToolModule] that exposes two tools:
//
//   - vuln_scan: run govulncheck on a Go module directory
//   - vuln_osv_query: query the OSV API for a specific module version
//
// # Example
//
//	s := vuln.NewScanner(vuln.ScannerConfig{Dir: "."})
//	result, err := s.Scan(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Summary)
package vuln
