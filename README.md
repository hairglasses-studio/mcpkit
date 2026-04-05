# mcpkit

[![Go Reference](https://pkg.go.dev/badge/github.com/hairglasses-studio/mcpkit.svg)](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/hairglasses-studio/mcpkit)](https://goreportcard.com/report/github.com/hairglasses-studio/mcpkit)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/hairglasses-studio/mcpkit/actions/workflows/ci.yml/badge.svg)](https://github.com/hairglasses-studio/mcpkit/actions/workflows/ci.yml)
[![MCP](https://img.shields.io/badge/MCP-2025--11--25-blue)](https://modelcontextprotocol.io/specification/2025-11-25)

The Go toolkit for production-grade MCP servers.

Built on [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go), mcpkit provides the middleware, type safety, and operational patterns needed to run MCP servers in production. It targets the [MCP 2025-11-25 spec](https://modelcontextprotocol.io/specification/2025-11-25) with 100% feature coverage.

## Features

- **100% MCP 2025-11-25 spec coverage** — tools, resources, prompts, sampling, logging, elicitation, structured output, async tasks
- **35+ packages across 4 dependency layers** — use only what you need; all packages are independently importable
- **90%+ test coverage across all packages** — comprehensive coverage with `-race` detection
- **Dual-SDK support** — works with mcp-go today; `//go:build official_sdk` tags enable migration to the official Go SDK without rewriting tool code
- **Typed handlers** — `TypedHandler[In, Out]` generates schemas from Go structs, populates `structuredContent`, and eliminates manual JSON wiring
- **Middleware chain** — composable middleware applied per-tool or globally; standard signature across all packages
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
- **Gateway** — multi-server aggregation with namespaced tool routing and per-upstream resilience policies (`gateway`)
- **Agent memory** — episodic/semantic/procedural memory tiers with pluggable storage backends (`memory`)
- **Skills** — context-aware lazy tool loading with skill bundles and triggers (`skills`)
- **Autonomous loops** — the Ralph Loop pattern for iterative, self-directing task execution (`ralph`)

## Quick Start

### Installation

```bash
go get github.com/hairglasses-studio/mcpkit@latest
```

### Stage 1: Hello World

A minimal MCP server with one typed tool. Create `main.go`:

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

Run it, then test with the [MCP Inspector](https://github.com/modelcontextprotocol/inspector):

```bash
go run main.go                                          # stdio server
npx @modelcontextprotocol/inspector go run main.go      # interactive debugger
```

**What you get:** `GreetInput` generates JSON Schema automatically from struct tags. The typed output is serialized as both `content[].text` (JSON) and `structuredContent`. The server speaks stdio JSON-RPC, the standard transport for local MCP servers.

### Stage 2: Add Middleware

Wrap every tool call with logging, rate limiting, and circuit breaking. Middleware is configured via the registry.

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "os"
    "time"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resilience"
)

type GreetInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to greet"`
}

type GreetOutput struct {
    Message string `json:"message"`
}

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil)) // never log to stdout

    reg := registry.NewToolRegistry(registry.Config{
        Middleware: []registry.Middleware{
            resilience.RateLimitMiddleware(resilience.NewRateLimitRegistry()),
            resilience.CircuitBreakerMiddleware(resilience.NewCircuitBreakerRegistry(nil)),
            loggingMiddleware(logger),
        },
    })
    reg.RegisterModule(&greetModule{})

    s := registry.NewMCPServer("greeter", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}

// greetModule implements registry.ToolModule.
type greetModule struct{}

func (m *greetModule) Name() string        { return "greet" }
func (m *greetModule) Description() string { return "Greeting tools" }
func (m *greetModule) Tools() []registry.ToolDefinition {
    return []registry.ToolDefinition{
        handler.TypedHandler[GreetInput, GreetOutput](
            "greet", "Greet a user by name",
            func(ctx context.Context, in GreetInput) (GreetOutput, error) {
                return GreetOutput{Message: "Hello, " + in.Name + "!"}, nil
            },
        ),
    }
}

func loggingMiddleware(logger *slog.Logger) registry.Middleware {
    return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
        return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
            start := time.Now()
            result, err := next(ctx, req)
            logger.Info("tool called", "tool", name, "duration", time.Since(start), "error", err != nil)
            return result, err
        }
    }
}
```

**What you get:** Rate limiting per `CircuitBreakerGroup`, circuit breaking after repeated failures, and structured logging to stderr. The middleware signature `func(name, td, next) handler` is the same across all mcpkit packages.

### Stage 3: Add Testing

Test tools in-process with `mcptest` -- no Inspector, no network, runs in milliseconds.

```go
// main_test.go
package main

import (
    "testing"

    "github.com/hairglasses-studio/mcpkit/mcptest"
    "github.com/hairglasses-studio/mcpkit/registry"
)

func TestGreetTool(t *testing.T) {
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&greetModule{})

    srv := mcptest.NewServer(t, reg)
    client := mcptest.NewClient(t, srv)

    result := client.CallTool("greet", map[string]any{"name": "World"})

    mcptest.AssertNotError(t, result)
    mcptest.AssertToolResultContains(t, result, "Hello, World!")
}

func TestGreetTool_MissingName(t *testing.T) {
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&greetModule{})

    srv := mcptest.NewServer(t, reg)
    client := mcptest.NewClient(t, srv)

    result := client.CallTool("greet", map[string]any{})

    mcptest.AssertError(t, result, "")
}
```

```bash
go test ./... -count=1
```

**What you get:** `mcptest.NewServer` creates a real MCP server in-process. `CallTool` sends a real `tools/call` request through the full handler chain including middleware. Tests run in ~10ms with no external dependencies.

### Stage 4: Module Pattern (Production)

For servers with many tools, use the `ToolModule` interface. Each module is a self-contained package with its own types, handlers, and tests.

```go
// tools/greet/module.go
package greet

import (
    "context"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

// Module implements registry.ToolModule.
type Module struct{}

func (m *Module) Name() string        { return "greet" }
func (m *Module) Description() string { return "Greeting tools" }

func (m *Module) Tools() []registry.ToolDefinition {
    return []registry.ToolDefinition{
        handler.TypedHandler[GreetInput, GreetOutput](
            "greet", "Greet a user by name", m.handleGreet,
        ),
        handler.TypedHandler[FarewellInput, GreetOutput](
            "farewell", "Say goodbye to a user", m.handleFarewell,
        ),
    }
}

type GreetInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to greet"`
}
type FarewellInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to bid farewell"`
}
type GreetOutput struct {
    Message string `json:"message"`
}

func (m *Module) handleGreet(ctx context.Context, in GreetInput) (GreetOutput, error) {
    return GreetOutput{Message: "Hello, " + in.Name + "!"}, nil
}

func (m *Module) handleFarewell(ctx context.Context, in FarewellInput) (GreetOutput, error) {
    return GreetOutput{Message: "Goodbye, " + in.Name + "!"}, nil
}
```

```go
// cmd/server/main.go
package main

import (
    "log"

    "github.com/hairglasses-studio/mcpkit/registry"
    "myserver/tools/greet"
)

func main() {
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&greet.Module{})

    s := registry.NewMCPServer("greeter", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

**What you get:** Each module is a self-contained package. The `ToolModule` interface (`Name()`, `Description()`, `Tools()`) is the only contract. This is the pattern used by dotfiles-mcp (90 tools), hg-mcp (1,190+ tools), and all production mcpkit servers.

### Stage 5: Gateway (Multi-Server Aggregation)

Aggregate multiple upstream MCP servers behind a single endpoint with per-upstream resilience.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hairglasses-studio/mcpkit/gateway"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resilience"
)

func main() {
    gw, reg := gateway.NewGateway()

    ctx := context.Background()

    gw.AddUpstream(ctx, gateway.UpstreamConfig{
        Name: "systemd",
        URL:  "http://localhost:8081/mcp",
        Policy: gateway.UpstreamPolicy{
            CircuitBreaker: &resilience.CircuitBreakerConfig{
                FailureThreshold: 5,
                Timeout:          time.Minute,
            },
        },
    })

    gw.AddUpstream(ctx, gateway.UpstreamConfig{
        Name: "process",
        URL:  "http://localhost:8082/mcp",
    })

    s := registry.NewMCPServer("ops-gateway", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

**What you get:** Tool namespacing (`systemd.systemd_status`, `process.ps_list`), per-upstream circuit breakers and rate limiting, and dynamic upstream registration. One failing server does not take down the gateway.

### Debugging

Test any mcpkit server with the MCP Inspector:

```bash
npx @modelcontextprotocol/inspector ./my-server                                    # compiled binary
npx @modelcontextprotocol/inspector go run ./cmd/server/                           # go run
npx @modelcontextprotocol/inspector --env API_KEY=test go run ./cmd/server/        # with env vars
```

For detailed tutorials and advanced topics, see the [full documentation on pkg.go.dev](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit).

## Package Map

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration, tool integrity verification | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
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
| `sampling` | Sampling client interface, context injection middleware, request builders | `registry` |
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

## Dependency Layers

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`
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
