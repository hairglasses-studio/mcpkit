# mcpkit — Gemini CLI Instructions

Production-grade MCP toolkit for Go. 35+ packages: registry, handlers, resilience, auth, security, observability, workflows, multi-agent orchestration. 100% MCP spec coverage.

## Build & Test

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests
make check               # All three above
```

## Architecture

4 dependency layers (lower never imports upper):
1. `registry`, `health`, `sanitize`, `secrets`, `client`, `transport`
2. `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `discovery`, `memory`, `finops`, `eval`, `session`
3. `security`, `gateway`, `ralph`, `skills`, `rdcycle`
4. `orchestrator`, `handoff`, `workflow`, `bootstrap`

## Key Conventions

- Middleware: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Params: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Results: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`
- Thread safety: `sync.RWMutex` on all registries
- Dual-SDK: build tags for mcp-go vs official go-sdk
- Tests: `*_test.go` in same package, 90%+ coverage
