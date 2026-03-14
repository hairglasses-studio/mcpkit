# mcpkit

MCP toolkit for building production-grade MCP servers. Built on `github.com/mark3labs/mcp-go`.

## Commands

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
```

## Package Map

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry generics, middleware | `registry` |
| `mcptest` | Test server/client, assertion helpers, HTTP pool | `registry` |
| `auth` | JWT/API key middleware, context identity | `registry` |
| `security` | RBAC, audit logging middleware | `registry`, `auth` |
| `health` | Health check endpoint and checker registry | none |
| `observability` | OpenTelemetry tracing/metrics middleware | `registry` |
| `sanitize` | Input sanitization for tool params | none |
| `secrets` | Secret provider interface, env/file providers, sanitizer | none |
| `client` | HTTP pool and client utilities | none |
| `resources` | Resource registry, middleware chain, server integration for URI-based data | none |
| `prompts` | Prompt registry, middleware chain, server integration for reusable templates | none |
| `research` | MCP ecosystem monitoring and viability assessment tools | `registry`, `handler`, `client` |

## Dependency Layers

- **Layer 1** (no internal deps): `registry`, `resources`, `prompts`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `handler`, `resilience`, `mcptest`, `auth`, `observability`, `research`
- **Layer 3** (depend on Layer 2): `security`

## Coding Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)` — codes defined in `handler/result.go`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`, `handler.StructuredResult()`
- Thread safety: all registries use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- SDK compat: import MCP types through `registry/compat.go` aliases when building tool modules

## Testing

- Test files: `*_test.go` in same package
- Use `mcptest.NewServer()` for integration tests, stdlib `testing` for unit tests
- Assertions: `mcptest/assert.go` helpers or stdlib `t.Errorf`/`t.Fatalf`
- Each package's tests must pass in isolation: `go test ./handler/ -count=1`

## Parallel Agent Rules

- **One agent per package** — don't edit files in another agent's package directory
- Import direction: lower layers never import upper layers (see Dependency Layers)
- Feature branches: each agent works on its own branch, merged after CI passes
- New package checklist: add CLAUDE.md if complex, ensure `_test.go` files exist, follow middleware signature if applicable

## Roadmap

Current spec coverage: **86%** (12/14 MCP 2025-11-25 features implemented or partial).

Next priorities:
1. Example servers — minimal, full-featured, and migration showcase
2. Official Go SDK migration path — compat.go update strategy, dual-SDK CI
3. Streamable HTTP verification tests

See [RESEARCH.md](RESEARCH.md) for full analysis: 17 roadmap items across 3 priority tiers, 4 implementation phases.
