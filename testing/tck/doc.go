// Package tck provides a Technology Compatibility Kit for mcpkit servers.
//
// The TCK validates that an MCP server built with mcpkit conforms to
// framework-level guarantees: tool registration, handler contracts,
// error code compliance, input schema validity, and lifecycle behavior.
// It runs as a standard Go test suite against any ToolRegistry instance.
//
// Usage:
//
//	reg := registry.NewToolRegistry()
//	reg.RegisterModule(&myModule{})
//	suite := tck.NewSuite(reg)
//	suite.Run(t)
package tck
