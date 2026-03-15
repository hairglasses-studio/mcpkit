# mcpkit

MCP toolkit for building production-grade MCP servers. Built on `github.com/mark3labs/mcp-go`.

## Commands

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
make build-official      # Verify official SDK build
make check-dual          # Full check + official SDK build
```

## Package Map

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry generics, middleware | `registry` |
| `mcptest` | Test server/client, assertion helpers, HTTP pool | `registry` |
| `auth` | JWT/JWKS validation, OAuth discovery + client flow, Bearer middleware, DPoP proof validation + HTTP middleware, context identity | `registry`, `client` |
| `security` | RBAC, audit logging middleware | `registry`, `auth` |
| `health` | Health check endpoint and checker registry | none |
| `observability` | OpenTelemetry tracing/metrics middleware | `registry` |
| `sanitize` | Input sanitization for tool params | none |
| `secrets` | Secret provider interface, env/file providers, sanitizer | none |
| `client` | HTTP pool and client utilities | none |
| `discovery` | MCP Registry client for server discovery and publishing | `registry`, `client` |
| `resources` | Resource registry, middleware chain, server integration for URI-based data | `registry` |
| `prompts` | Prompt registry, middleware chain, server integration for reusable templates | `registry` |
| `logging` | slog.Handler bridge to MCP clients, tool invocation logging middleware | `registry` |
| `sampling` | Sampling client interface, context injection middleware, request builders | `registry` |
| `roots` | Client workspace root discovery, caching, context helpers | `registry` |
| `research` | MCP ecosystem monitoring and viability assessment tools | `registry`, `handler`, `client` |
| `gateway` | Multi-server aggregation with namespaced tool routing | `registry`, `client` |
| `ralph` | Autonomous loop runner for iterative task execution (Ralph Loop pattern) | `registry`, `handler`, `sampling` |

## Dependency Layers

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`

## Coding Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)` — codes defined in `handler/result.go`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`, `handler.StructuredResult()`
- Thread safety: all registries use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- SDK compat: import MCP types through `registry/compat.go` aliases when building tool modules
- Dual-SDK: `//go:build !official_sdk` tags on mcp-go specific files; `//go:build official_sdk` for go-sdk variants
- Adapter functions: use `registry.MakeTextContent()`, `registry.MakeErrorResult()`, `registry.ExtractArguments()` instead of SDK-specific constructors

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

Current spec coverage: **100%** (all MCP 2025-11-25 features implemented).
All Tier 1 and Tier 2 roadmap items complete.

Completed Tier 3 items:
- [x] Registry integration (MCP Registry for server discovery/publishing) — `discovery` package
- [x] Gateway pattern (multi-server aggregation and routing) — `gateway` package

Completed Tier 3 items (continued):
- [x] Playbooks / autonomous loops (Ralph Loop pattern) — `ralph` package

Remaining Tier 3 priorities:
1. Workload identity (GCP/AWS IAM for server-to-server auth)

### Future `ralph` Improvements
1. Multi-tool decisions — allow LLM to request multiple tool calls per iteration (parallel execution)
2. Spec validation — schema validation for spec files, required field checks, task ID uniqueness
3. Resumable loops — resume from progress file after process restart (re-attach to in-flight state)
4. Event hooks — user-defined callbacks on iteration start/end, task completion, errors
5. Conditional tasks — task dependencies and prerequisite chains (DAG-based execution)
6. Streaming progress — SSE/websocket push of iteration status to connected clients
7. Cost tracking — token usage accounting per iteration and cumulative per loop run
8. Spec templates — YAML support, variable interpolation, spec composition from fragments

See [RESEARCH.md](RESEARCH.md) for full analysis: 17 roadmap items across 3 priority tiers, 4 implementation phases.
