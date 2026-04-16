Dual-SDK compatibility (mcp-go default, official go-sdk optional):
- Use `//go:build !official_sdk` on mcp-go-specific files
- Use `//go:build official_sdk` for go-sdk variants
- Always import MCP types through `registry/compat.go` aliases, not SDK constructors
- Use adapter functions: `registry.MakeTextContent()`, `registry.MakeErrorResult()`,
  `registry.ExtractArguments()` — never call SDK-specific constructors directly
- `make check-dual` must pass before merging changes to registry or handler packages
