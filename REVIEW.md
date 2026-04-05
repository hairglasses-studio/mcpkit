# Review Guidelines — mcpkit

Inherits from org-wide [REVIEW.md](https://github.com/hairglasses-studio/.github/blob/main/REVIEW.md).

## Additional Focus
- **Middleware chain**: Verify middleware signature matches `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- **Registry thread safety**: All registry access must use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- **Handler contract**: Always return `(*mcp.CallToolResult, nil)` — never `(nil, error)`
- **Backward compatibility**: Public API changes must not break downstream MCP servers (6+ dependents)
- **Test coverage**: New handlers need both unit tests (stdlib) and integration tests (`mcptest.NewServer()`)
