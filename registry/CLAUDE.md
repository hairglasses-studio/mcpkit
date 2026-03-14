# registry

Core tool registry — no internal dependencies. All other packages depend on this.

## Key Types

- `ToolDefinition` — complete tool: `mcp.Tool` + metadata (category, tags, complexity, timeout, circuit breaker group)
- `ToolModule` — interface: `Name()`, `Description()`, `Tools() []ToolDefinition`
- `Middleware` — `func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc`
- `ToolRegistry` — thread-safe registry with `sync.RWMutex`

## Patterns

- **Compat layer** (`compat.go`): type aliases for `mcp-go` types — when official SDK ships, update only this file
- **Deferred tools** (`deferred.go`): `RegisterDeferredModule` + `ListEagerTools`/`ListDeferredTools` for lazy loading
- **Dynamic tools** (`dynamic.go`): runtime tool registration/removal
- **Search** (`search.go`): fuzzy tool search across name, description, tags, category
- **Annotations** (`annotations.go`): auto-generates `mcp.ToolAnnotation` from `ToolDefinition` metadata
- **Middleware chain**: applied in `wrapHandler` — user middleware wraps inner, then timeout/panic/truncation applied outermost
