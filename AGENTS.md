# mcpkit — Agent Instructions

Production-grade MCP toolkit for building MCP servers in Go. 35+ packages covering registry, handlers, resilience, auth, security, observability, workflows, and multi-agent orchestration. 100% MCP 2025-11-25 spec coverage.

## Build & Test

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
make build-official      # Verify official SDK build
make check-dual          # Full check + official SDK build
```

## Architecture (Dependency Layers)

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`, `transport`
- **Layer 2**: `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`, `session`, `feedback`
- **Layer 3**: `security`, `gateway`, `ralph`, `skills`, `rdcycle`, `cmd`
- **Layer 4**: `orchestrator`, `handoff`, `workflow`, `bootstrap`

## Key Packages

| Package | Purpose |
|---------|---------|
| `registry` | Tool registration, middleware chain, server integration |
| `handler` | TypedHandler generics, param extraction, result builders |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry, middleware |
| `mcptest` | Test server/client, assertions, replay, snapshot, benchmarks |
| `auth` | JWT/JWKS, OAuth, DPoP, workload identity |
| `security` | RBAC, audit logging, tenant propagation |
| `gateway` | Multi-server aggregation, namespaced routing, per-upstream resilience |
| `workflow` | Cyclical graph engine, state machines, checkpoints, compensation |
| `ralph` | Autonomous loop runner (Ralph Loop pattern) |
| `orchestrator` | Fan-out, pipeline, select patterns |
| `handoff` | Agent delegation protocol |

## Coding Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`
- Thread safety: all registries use `sync.RWMutex`
- SDK compat: import MCP types through `registry/compat.go` aliases
- Dual-SDK: `//go:build !official_sdk` / `//go:build official_sdk` build tags
- Adapter functions: `registry.MakeTextContent()`, `registry.MakeErrorResult()`, `registry.ExtractArguments()`

## Testing

- Test files: `*_test.go` in same package
- Use `mcptest.NewServer()` for integration tests
- Each package's tests must pass in isolation: `go test ./handler/ -count=1`
- All 35 packages at 90%+ coverage

## Parallel Agent Rules

- One agent per package — don't edit files in another agent's package directory
- Import direction: lower layers never import upper layers
- Feature branches: each agent works on its own branch
- New package checklist: add docs, ensure `_test.go` files exist, follow middleware signature
