# mcpkit

[![Go Reference](https://pkg.go.dev/badge/github.com/hairglasses-studio/mcpkit.svg)](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/hairglasses-studio/mcpkit)](https://goreportcard.com/report/github.com/hairglasses-studio/mcpkit)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/hairglasses-studio/mcpkit/actions/workflows/ci.yml/badge.svg)](https://github.com/hairglasses-studio/mcpkit/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/Coverage-85%25%2B-brightgreen.svg)](https://github.com/hairglasses-studio/mcpkit)
[![MCP](https://img.shields.io/badge/MCP-2025--11--25-blue)](https://modelcontextprotocol.io/specification/2025-11-25)

The Go toolkit for production-grade MCP servers.

Built on [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go), mcpkit provides the middleware, type safety, and operational patterns needed to run MCP servers in production. It targets the [MCP 2025-11-25 spec](https://modelcontextprotocol.io/specification/2025-11-25) with 100% feature coverage.

## Features

- **100% MCP 2025-11-25 spec coverage** — tools, resources, prompts, sampling, logging, elicitation, structured output, async tasks
- **36 packages across 4 dependency layers** — use only what you need; all packages are independently importable
- **85%+ test coverage across all packages** — comprehensive coverage with `-race` detection
- **Dual-SDK support** — works with mcp-go today; `//go:build official_sdk` tags enable migration to the official Go SDK without rewriting tool code
- **Typed handlers** — `TypedHandler[In, Out]` generates schemas from Go structs, populates `structuredContent`, and eliminates manual JSON wiring
- **Middleware chain** — composable middleware applied per-tool or globally; standard signature across all packages
- **Response truncation** — configurable byte-budget middleware that caps oversized tool responses and appends guidance messages (`middleware/truncate`)
- **Production resilience** — circuit breakers, rate limiters, and caching via the `resilience` package
- **Auth** — JWT/JWKS validation, OAuth 2.1 discovery and client flow, Bearer middleware, DPoP proof validation, workload identity (GCP/AWS)
- **RBAC and audit logging** — role-based tool access control and structured audit trails via the `security` package
- **Tenant isolation** — tenant context propagation middleware for multi-tenant servers
- **Multi-agent orchestration** — fan-out, pipeline, and select patterns (`orchestrator`); manager/agent-as-tool delegation (`handoff`)
- **Workflow engine** — cyclical graph execution with conditional branching, checkpoints, and state machines (`workflow`)
- **Cost management** — token accounting, budget policies, dollar-cost estimation, scoped per-tenant/user/session budgets, time-windowed tracking (`finops`)
- **Testing infrastructure** — test server/client, assertion helpers, session record/replay, golden file snapshots, benchmark helpers (`mcptest`)
- **Input/output sanitization** — secret and PII redaction, injection filtering, URI validation with SSRF and path traversal protection (`sanitize`)
- **Tool integrity** — SHA-256 fingerprinting and tamper detection for registered tools (`registry`)
- **GenAI-aware observability** — tool spans, token usage bridging, and client-side sampling spans for Anthropic-compatible and native Ollama backends (`observability`, `sampling`)
- **Gateway** — multi-server aggregation with namespaced tool routing and per-upstream resilience policies (`gateway`)
- **Agent memory** — episodic/semantic/procedural memory tiers with pluggable storage backends (`memory`)
- **Skills** — context-aware lazy tool loading with skill bundles and triggers (`skills`)
- **Repo front doors** — generated skill priority surface for the framework workflows ([docs/SKILL-FRONT-DOORS.md](docs/SKILL-FRONT-DOORS.md))
- **Autonomous loops** — the Ralph Loop pattern for iterative, self-directing task execution (`ralph`)
- **MCP-A2A bridge** — bidirectional MCP/A2A protocol bridge: expose MCP tools as A2A skills and consume A2A agents as MCP tools (`bridge/a2a`) ([docs](bridge/a2a/README.md))
- **Multi-protocol gateway** — single HTTP endpoint serving MCP, A2A, and OpenAI function calling via automatic protocol detection and canonical translation (`gateway/multi`) ([docs](gateway/multi/README.md))

## Quick Start

```bash
go get github.com/hairglasses-studio/mcpkit@latest
```

```go
package main

import (
    "context"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type GreetInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to greet"`
}

type GreetOutput struct {
    Message string `json:"message"`
}

func main() {
    td := handler.TypedHandler[GreetInput, GreetOutput](
        "greet", "Greet a user by name",
        func(ctx context.Context, in GreetInput) (GreetOutput, error) {
            return GreetOutput{Message: "Hello, " + in.Name + "!"}, nil
        },
    )

    s := registry.NewMCPServer("greeter", "1.0.0")
    registry.AddToolToServer(s, td.Tool, td.Handler)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

```bash
go run main.go                                          # stdio server
npx @modelcontextprotocol/inspector go run main.go      # interactive debugger
```

**[QUICKSTART.md](QUICKSTART.md)** has the full 5-stage progressive tutorial: hello world, typed parameters, middleware, resources and prompts, and testing with `mcptest`.

## Package Map

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration, tool integrity verification | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
| `middleware/truncate` | Response size limiting with byte budgets, configurable guidance messages, error passthrough | `registry` |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry generics, middleware | `registry` |
| `mcptest` | Test server/client, assertion helpers, HTTP pool, session replay, snapshot testing, benchmark helpers | `registry` |
| `auth` | JWT/JWKS validation, OAuth discovery + client flow, Bearer middleware, DPoP proof validation + HTTP middleware, workload identity (GCP/AWS), context identity | `registry`, `client` |
| `security` | RBAC, audit logging middleware, tenant context propagation | `registry`, `auth` |
| `health` | Health check endpoint and checker registry | none |
| `observability` | OpenTelemetry tracing/metrics middleware | `registry` |
| `sanitize` | Input/output sanitization, secret/PII redaction, URI validation | none |
| `secrets` | Secret provider interface, env/file providers, sanitizer | none |
| `client` | HTTP pool and client utilities | none |
| `discovery` | MCP Registry client for server discovery and publishing, multi-registry metadata extraction | `registry`, `client`, `resources`, `prompts` |
| `resources` | Resource registry, middleware chain, server integration for URI-based data, URI validation middleware | `registry` |
| `prompts` | Prompt registry, middleware chain, server integration for reusable templates | `registry` |
| `logging` | slog.Handler bridge to MCP clients, tool invocation logging middleware | `registry` |
| `sampling` | Sampling client interface, context injection middleware, request builders, GenAI client spans | `registry`, `finops` |
| `roots` | Client workspace root discovery, caching, context helpers | `registry` |
| `research` | MCP ecosystem monitoring and viability assessment tools | `registry`, `handler`, `client` |
| `gateway` | Multi-server aggregation with namespaced tool routing, per-upstream resilience (circuit breaker, rate limit, timeout) | `registry`, `client`, `resilience` |
| `dispatcher` | Priority worker pool with concurrency groups, middleware integration | `registry` |
| `ralph` | Autonomous loop runner for iterative task execution (Ralph Loop pattern) | `registry`, `handler`, `sampling`, `finops` |
| `finops` | Token accounting, budget policies, usage tracking middleware, dollar-cost estimation, scoped budgets, time-windowed tracking | `registry` |
| `memory` | Agent memory registry with pluggable storage backends | `registry` |
| `skills` | Context-aware lazy tool loading with skill bundles and triggers | `registry` |
| `handoff` | Agent delegation protocol with manager/agent-as-tool patterns, delegate middleware | `registry`, `sampling`, `finops` |
| `orchestrator` | Multi-agent execution patterns: fan-out, pipeline, select, stage middleware | none |
| `workflow` | Cyclical graph engine with conditional branching, checkpoints, state machines, node middleware | `orchestrator`, `registry`, `sampling` |
| `extensions` | MCP Extensions negotiation and capability handshake | none |
| `lifecycle` | Production server lifecycle: signal handling, graceful drain, shutdown hooks | none |
| `bootstrap` | Agent workspace init, context reports, capability matrix | `registry`, `resources`, `prompts`, `extensions` |
| `eval` | Evaluation framework: cases, scorers, JSON suite loading, runner | `registry` |
| `roadmap` | Machine-readable roadmap management, gap analysis, query functions | `registry`, `handler` |
| `rdcycle` | R&D cycle orchestration tools: scan, plan, verify, commit, report | `registry`, `handler`, `research`, `roadmap`, `workflow`, `finops` |
| `bridge/a2a` | Bidirectional MCP/A2A bridge: tool-to-skill translation, agent card generation, bridge executor, remote agent consumer | `registry`, `handler` |
| `gateway/multi` | Multi-protocol HTTP gateway: MCP, A2A, and OpenAI adapters with auto-detection and canonical translation | `registry` |

## Dependency Layers

```
Layer 4  orchestrator ─ handoff ─ workflow ─ bootstrap
            │              │          │
Layer 3  security ── gateway ── ralph ── skills ── rdcycle
            │           │         │                  │
Layer 2  handler ─ auth ─ resilience ─ mcptest ─ finops ─ eval
         resources ─ prompts ─ discovery ─ sampling ─ ...
            │           │           │
Layer 1  registry ── health ── sanitize ── secrets ── client
         (no internal dependencies)
```

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `middleware/truncate`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`, `gateway/multi`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `rdcycle`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`

Lower layers never import upper layers. All packages in a layer can be used independently.

## Commands

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
make build-official      # Verify official SDK build
make check-dual          # Full check + official SDK build
```

## License

See [LICENSE](LICENSE) for details.
