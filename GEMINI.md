# mcpkit — Gemini CLI Instructions

MCP toolkit for building production-grade MCP servers. Built on `github.com/mark3labs/mcp-go`.

## Build & Test

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
make build-official      # Verify official SDK build
make check-dual          # Full check + official SDK build
```

## Architecture


## Key Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)` — codes defined in `handler/result.go`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`, `handler.StructuredResult()`
- Thread safety: all registries use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- SDK compat: import MCP types through `registry/compat.go` aliases when building tool modules
- Dual-SDK: `//go:build !official_sdk` tags on mcp-go specific files; `//go:build official_sdk` for go-sdk variants
- Adapter functions: use `registry.MakeTextContent()`, `registry.MakeErrorResult()`, `registry.ExtractArguments()` instead of SDK-specific constructors


## Shared Research Repository

Cross-project research lives at `~/hairglasses-studio/docs/` (git: hairglasses-studio/docs). When launching research agents, check existing docs first and write reusable research outputs back to the shared repo rather than local docs/.
