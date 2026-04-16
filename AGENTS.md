# mcpkit — Agent Instructions

> Canonical instructions: AGENTS.md

Use this file as the canonical instruction surface for Codex-first repo guidance. [CLAUDE.md](CLAUDE.md) and [GEMINI.md](GEMINI.md) are compatibility mirrors.

MCP toolkit for building production-grade MCP servers. Built on `github.com/mark3labs/mcp-go`.

## Codex Surfaces

- Canonical workflow skill: `.agents/skills/mcpkit/SKILL.md`
- Shared subagents: `.codex/agents/`

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

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration, tool integrity verification | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry generics, middleware | `registry` |
| `mcptest` | Test server/client, assertion helpers, HTTP pool, session replay, snapshot testing, benchmark helpers | `registry` |
| `auth` | JWT/JWKS validation, OAuth discovery + client flow, Bearer middleware, DPoP proof validation + HTTP middleware, workload identity (GCP/AWS), context identity | `registry`, `client` |
| `security` | RBAC, audit logging middleware, audit export (JSONL/stream), tenant context propagation | `registry`, `auth` |
| `health` | Health check endpoint and checker registry | none |
| `observability` | OpenTelemetry tracing/metrics middleware | `registry` |
| `sanitize` | Input/output sanitization, secret/PII redaction, URI validation | none |
| `secrets` | Secret provider interface, env/file providers, sanitizer | none |
| `client` | HTTP pool and client utilities | none |
| `discovery` | MCP Registry client for server discovery and publishing, multi-registry metadata extraction, server card HTTP handler | `registry`, `client`, `resources`, `prompts` |
| `resources` | Resource registry, middleware chain, server integration for URI-based data, URI validation middleware | `registry` |
| `prompts` | Prompt registry, middleware chain, server integration for reusable templates | `registry` |
| `logging` | slog.Handler bridge to MCP clients, tool invocation logging middleware | `registry` |
| `sampling` | Sampling client interface, context injection middleware, request builders | `registry` |
| `roots` | Client workspace root discovery, caching, context helpers | `registry` |
| `research` | MCP ecosystem monitoring and viability assessment tools, GitHub activity monitoring, diff analysis | `registry`, `handler`, `client` |
| `gateway` | Multi-server aggregation with namespaced tool routing, per-upstream resilience (circuit breaker, rate limit, timeout), dynamic upstream registration | `registry`, `client`, `resilience` |
| `dispatcher` | Priority worker pool with concurrency groups, middleware integration | `registry` |
| `ralph` | Autonomous loop runner for iterative task execution (Ralph Loop pattern), workflow-backed loop | `registry`, `handler`, `sampling`, `finops`, `workflow` |
| `finops` | Token accounting, budget policies, usage tracking middleware, dollar-cost estimation, scoped budgets, time-windowed tracking | `registry` |
| `memory` | Agent memory registry with pluggable storage backends | `registry` |
| `skills` | Context-aware lazy tool loading with skill bundles and triggers | `registry` |
| `handoff` | Agent delegation protocol with manager/agent-as-tool patterns, delegate middleware | `registry`, `sampling`, `finops` |
| `orchestrator` | Multi-agent execution patterns: fan-out, pipeline, select, stage middleware | none |
| `workflow` | Cyclical graph engine with conditional branching, checkpoints, state machines, node middleware, fork nodes for parallel branches, compensation/saga rollback | `orchestrator`, `registry`, `sampling` |
| `extensions` | MCP Extensions negotiation and capability handshake | none |
| `lifecycle` | Production server lifecycle: signal handling, graceful drain, shutdown hooks | none |
| `bootstrap` | Agent workspace init, context reports, capability matrix | `registry`, `resources`, `prompts`, `extensions` |
| `eval` | Evaluation framework: cases, scorers (exact/contains/regex/jsonpath/custom/not-empty/latency), JSON suite loading, runner | `registry` |
| `roadmap` | Machine-readable roadmap management, XML-tagged markdown rendering, gap analysis, query functions | `registry`, `handler` |
| `rdcycle` | R&D cycle orchestration tools: scan, plan, verify, commit, report, schedule, notes, improve, workflow graph, budget profiles, model tiers | `registry`, `handler`, `research`, `roadmap`, `workflow`, `finops` |
| `session` | Session management: Session/SessionStore interfaces, middleware, TTL/eviction, migration | `registry` |
| `transport` | Transport abstraction layer: stdio, HTTP, WebSocket adapters, middleware chain | none |
| `feedback` | User feedback collection, anonymous telemetry, opt-in usage tracking | `registry`, `handler` |
| `cmd` | CLI helpers for server publishing, configuration, management | `discovery`, `registry` |

## Key Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)` — codes defined in `handler/result.go`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`, `handler.StructuredResult()`
- Thread safety: all registries use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- SDK compat: import MCP types through `registry/compat.go` aliases when building tool modules
- Dual-SDK: `//go:build !official_sdk` tags on mcp-go specific files; `//go:build official_sdk` for go-sdk variants
- Adapter functions: use `registry.MakeTextContent()`, `registry.MakeErrorResult()`, `registry.ExtractArguments()` instead of SDK-specific constructors
