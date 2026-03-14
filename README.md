# mcpkit

The Go toolkit for production-grade MCP servers.

Built on [mcp-go](https://github.com/mark3labs/mcp-go), mcpkit adds the middleware, type safety, and operational patterns needed to run MCP servers in production. It targets the [MCP 2025-11-25 spec](https://modelcontextprotocol.io/specification/2025-11-25) with 72% feature coverage and growing.

## Features

- **Typed Handlers** — Define tool inputs/outputs as Go structs. Schemas are generated automatically via `jsonschema` tags. No manual JSON wiring.
- **Middleware Chain** — Composable middleware for auth, rate limiting, circuit breaking, observability, and more. Applied per-tool or globally.
- **Registry & Search** — Thread-safe tool registry with category/tag organization and fuzzy search across names, descriptions, and metadata.
- **Structured Output** — `TypedHandler[In, Out]` auto-generates `outputSchema` and populates both `structuredContent` and text content per the 2025-11-25 spec.
- **Elicitation** — First-class support for requesting user input during tool execution via `ElicitForm`, `ElicitURL`, and `ElicitFormSchema`.
- **Resilience** — Circuit breakers, rate limiters, and caching as generic middleware. Configure per-tool or per-group.
- **Auth & Security** — JWT/API key validation, RBAC with role-based tool access, and audit logging middleware.
- **Observability** — OpenTelemetry tracing and metrics middleware. Drop-in integration with any OTLP-compatible backend.
- **SDK Abstraction** — `registry/compat.go` type aliases isolate your code from SDK changes. When the official Go SDK stabilizes, update one file.
- **Test Infrastructure** — `mcptest` package with test servers, assertion helpers, and HTTP connection pooling for fast integration tests.

## Installation

```bash
go get github.com/hairglasses-studio/mcpkit
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/mark3labs/mcp-go/server"
)

type GreetInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to greet"`
}

type GreetOutput struct {
    Message string `json:"message"`
}

func main() {
    // Create a typed tool
    tool := handler.TypedHandler[GreetInput, GreetOutput](
        "greet",
        "Greet a user by name",
        func(ctx context.Context, input GreetInput) (GreetOutput, error) {
            return GreetOutput{Message: "Hello, " + input.Name + "!"}, nil
        },
    )

    // Register and serve
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&singleToolModule{tool: tool})

    s := server.NewMCPServer("greeter", "1.0.0")
    reg.RegisterWithServer(s)

    if err := server.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}

type singleToolModule struct{ tool registry.ToolDefinition }

func (m *singleToolModule) Name() string                  { return "greeter" }
func (m *singleToolModule) Description() string           { return "Greeting tools" }
func (m *singleToolModule) Tools() []registry.ToolDefinition { return []registry.ToolDefinition{m.tool} }
```

## Packages

| Package | Purpose |
|---------|---------|
| `registry` | Tool registration, middleware chain, server integration, SDK compat layer |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation |
| `resilience` | Circuit breaker, rate limiter, cache — all as composable middleware |
| `mcptest` | Test server/client, assertion helpers, HTTP connection pool |
| `auth` | JWT and API key validation middleware, context-based identity |
| `security` | RBAC (role-based access control) and audit logging middleware |
| `health` | Health check endpoint and checker registry |
| `observability` | OpenTelemetry tracing and metrics middleware |
| `sanitize` | Input sanitization for tool parameters |
| `secrets` | Secret provider interface with env and file backends |
| `client` | HTTP connection pool and client utilities |

## Architecture

```
Layer 3:  security
              ↓
Layer 2:  handler  resilience  mcptest  auth  observability
              ↓         ↓         ↓       ↓        ↓
Layer 1:  registry   health   sanitize  secrets  client
```

Lower layers never import upper layers. All packages in a layer can be used independently.

## Feature Matrix

| Feature | Status | Spec Version |
|---------|--------|-------------|
| Tools (registration, middleware, search) | Implemented | Draft |
| Tool Annotations | Implemented | 2025-03-26 |
| Structured Output (outputSchema) | Implemented | 2025-11-25 |
| Elicitation | Implemented | 2025-11-25 |
| Tasks (async operations) | Implemented | 2025-11-25 |
| Deferred Tool Loading | Implemented | 2025-11-25 |
| OAuth 2.1 | Partial | 2025-03-26 |
| Streamable HTTP | Delegated (mcp-go) | 2025-03-26 |
| Resources | Planned | Draft |
| Prompts | Planned | Draft |
| Sampling | Planned | Draft |
| Logging | Planned | Draft |

See [RESEARCH.md](RESEARCH.md) for the full roadmap with 17 items across 3 priority tiers.

## Links

- [MCP Specification (2025-11-25)](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP Roadmap](https://modelcontextprotocol.io/development/roadmap)
- [mcp-go](https://github.com/mark3labs/mcp-go) — underlying Go SDK
- [Official Go SDK](https://github.com/modelcontextprotocol/go-sdk) — future migration target
- [FastMCP](https://gofastmcp.com) — Python framework (comparison reference)

## License

See [LICENSE](LICENSE) for details.
