# MCP Server Deployment Patterns: Migration Guide

When building an MCP server with mcpkit, three deployment patterns cover almost every scenario. This guide helps you pick the right one, shows production-ready code for each, and includes a decision matrix so you can make the call quickly.

---

## Pattern A: `.mcp.json` Inline Configuration

The lightest path. No build step, no Go code — register an existing binary or command directly in the MCP client config. Use this when your toolset is small (under 20 tools), you are distributing a pre-built binary, or you want zero-friction onboarding.

### What it looks like

A `.mcp.json` at the repo root (or Claude's global `mcpServers` config) tells the MCP host how to launch your server:

```json
{
  "mcpServers": {
    "my-tools": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "/absolute/path/to/cmd/my-tools/main.go"],
      "env": {
        "MY_API_KEY": "${MY_API_KEY}"
      }
    }
  }
}
```

For a compiled binary:

```json
{
  "mcpServers": {
    "my-tools": {
      "type": "stdio",
      "command": "/usr/local/bin/my-tools"
    }
  }
}
```

The underlying server is still a standard mcpkit binary — this config just tells the host how to start it. The minimal server it points to looks like:

```go
//go:build !official_sdk

package main

import (
    "context"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type EchoInput struct {
    Message string `json:"message" jsonschema:"required,description=Message to echo"`
}

type EchoOutput struct {
    Reply string `json:"reply"`
}

func main() {
    td := handler.TypedHandler[EchoInput, EchoOutput](
        "echo", "Echo a message back to the caller",
        func(_ context.Context, in EchoInput) (EchoOutput, error) {
            return EchoOutput{Reply: in.Message}, nil
        },
    )

    s := registry.NewMCPServer("my-tools", "1.0.0")
    registry.AddToolToServer(s, td.Tool, td.Handler)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

See `examples/minimal/main.go` for a working two-tool version of this pattern.

### Pros

- Zero infrastructure — the MCP host spawns the process directly
- No build pipeline required during development (`go run` works)
- Config lives next to the code; reviewable in PRs
- Absolute paths and `${ENV_VAR}` substitution are well-supported by Claude Code and Codex

### Cons

- All tools load into the LLM context at startup — expensive at 20+ tools
- No auth or rate-limiting surface without adding middleware in the binary itself
- Not suitable for shared/multi-user deployments
- `go run` cold-start latency (~1s) shows up on every session open

### When to choose Pattern A

- Personal toolsets: file operations, note search, code utilities (under 20 tools)
- CI/CD helpers that only one engineer runs locally
- Prototyping before committing to a proper server package
- Distributing a compiled binary to users who run it locally

---

## Pattern B: Discovery-First Contract Layer

Register a thin "front door" with the MCP host that exposes only catalog/search tools eagerly. All domain tools stay deferred — they are discovered through search and loaded only when needed. Use this when your toolset is large (50+ tools) and context budget matters.

This pattern uses mcpkit's `toolindex` package for the catalog/search front door, `registry.RegisterDeferredModule` for lazy loading, and optionally `skills.SkillRegistry` for trigger-based activation.

### Structure

```
cmd/my-server/
    main.go          ← server binary; registers modules
    .mcp.json        ← registers this binary with the MCP host
pkg/
    analytics/       ← analytics tool module (deferred)
    reporting/       ← reporting tool module (deferred)
    admin/           ← admin tool module (deferred)
```

### Contract layer main.go

```go
//go:build !official_sdk

package main

import (
    "log"

    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/toolindex"

    "github.com/my-org/my-server/pkg/analytics"
    "github.com/my-org/my-server/pkg/reporting"
    "github.com/my-org/my-server/pkg/admin"
)

func main() {
    reg := registry.NewToolRegistry()

    // Register domain modules with deferred loading.
    // Listed tool names stay out of the initial tools/list response
    // but remain discoverable via search.
    reg.RegisterDeferredModule(&analytics.Module{}, map[string]bool{
        "analytics_query":  true,
        "analytics_export": true,
        "analytics_trends": true,
    })
    reg.RegisterDeferredModule(&reporting.Module{}, map[string]bool{
        "report_generate": true,
        "report_schedule": true,
        "report_list":     true,
    })
    reg.RegisterDeferredModule(&admin.Module{}, map[string]bool{
        "admin_users_list":   true,
        "admin_users_invite": true,
    })

    // Register the discovery front door EAGERLY.
    // This exposes {prefix}_tool_catalog and {prefix}_tool_search to the LLM.
    reg.RegisterModule(toolindex.NewToolIndexModule("my_server", reg))

    s := registry.NewMCPServer("my-server", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

### What the LLM sees at startup

With 50 tools registered and all domain tools deferred, the initial `tools/list` response contains only two tools:

- `my_server_tool_catalog` — lists all tools grouped by category
- `my_server_tool_search` — fuzzy search across names, descriptions, tags

The LLM calls `my_server_tool_catalog` or `my_server_tool_search` first to find what it needs, then receives the full schema for the matching tools. Context usage drops by up to 85% compared to registering everything eagerly.

### Adding trigger-based skill loading

For large servers where tools cluster into coherent workflows, the `skills` package activates groups on demand:

```go
//go:build !official_sdk

package main

import (
    "context"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/skills"
    "github.com/hairglasses-studio/mcpkit/toolindex"
)

func main() {
    dynReg := registry.NewDynamicRegistry()
    // ... register all modules into dynReg ...

    sr := skills.NewSkillRegistry(dynReg)

    // Activate the "reporting" skill when the user mentions reports.
    sr.Register(skills.Skill{
        Name:        "reporting",
        Description: "Report generation and scheduling tools",
        Tools:       []string{"report_generate", "report_schedule", "report_list"},
        Trigger: func(ctx skills.SkillContext) bool {
            for _, hint := range ctx.TaskHints {
                if hint == "reports" || hint == "export" {
                    return true
                }
            }
            return false
        },
        Priority: 10,
    })

    // Expose a skill-activation tool so the LLM can explicitly load skill groups.
    activateTool := handler.TypedHandler[activateInput, activateOutput](
        "my_server_load_skill",
        "Load a named skill group to access its tools. Call my_server_tool_catalog first.",
        func(ctx context.Context, in activateInput) (activateOutput, error) {
            if err := sr.Activate(ctx, in.Skill); err != nil {
                return activateOutput{}, err
            }
            return activateOutput{Loaded: in.Skill}, nil
        },
    )
    dynReg.AddTool(activateTool)

    reg := dynReg.ToolRegistry()
    reg.RegisterModule(toolindex.NewToolIndexModule("my_server", reg))

    s := registry.NewMCPServer("my-server", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}

type activateInput struct {
    Skill string `json:"skill" jsonschema:"required,description=Skill name to activate"`
}
type activateOutput struct {
    Loaded string `json:"loaded"`
}
```

### Pros

- Context budget scales to hundreds of tools — only catalog/search tokens are charged upfront
- No extra infrastructure; still a single stdio binary
- Deferred loading is per-tool, not per-module — fine-grained control
- `toolindex` catalog groups by category, making large toolsets navigable

### Cons

- Adds one round-trip (search → load) before the LLM can use domain tools
- Requires careful tool naming and category tagging to make search work well
- `RegisterDeferredModule` does not gate execution — deferred tools are still callable once the server starts; the deferral is a context-budget concern, not an auth concern

### When to choose Pattern B

- Servers with 20–500+ tools where context overhead is a bottleneck
- Toolsets that cluster into coherent domains (analytics, admin, reporting, code review...)
- Team-internal servers where trust is assumed but token budgets matter
- The "ralphglasses" pattern: 200+ tools across 30 deferred groups, front door exposes only catalog/search/load

---

## Pattern C: Standalone Sidecar Package

A dedicated binary with its own module path, packaged for multi-user deployment, with auth, rate-limiting, and health infrastructure baked in. Use this when the server is shared infrastructure — CI/CD services, team APIs, SaaS agents — where users are untrusted, rate limits matter, and the server must survive process restarts.

### Project layout

```
my-org/my-mcp-server/
    go.mod               ← module github.com/my-org/my-mcp-server
    main.go              ← entry point
    module/
        module.go        ← registry.ToolModule implementation
    .well-known/
        mcp.json         ← generated server card (CI artifact)
    Dockerfile
    .mcp.json            ← for local development only
```

### Sidecar main.go

```go
//go:build !official_sdk

package main

import (
    "context"
    "errors"
    "flag"
    "log"
    "log/slog"
    "net/http"
    "time"

    "github.com/mark3labs/mcp-go/server"

    "github.com/hairglasses-studio/mcpkit/auth"
    "github.com/hairglasses-studio/mcpkit/discovery"
    "github.com/hairglasses-studio/mcpkit/finops"
    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/health"
    "github.com/hairglasses-studio/mcpkit/lifecycle"
    "github.com/hairglasses-studio/mcpkit/logging"
    "github.com/hairglasses-studio/mcpkit/middleware/truncate"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resilience"
    "github.com/hairglasses-studio/mcpkit/toolindex"

    "github.com/my-org/my-mcp-server/module"
)

func main() {
    contractWrite := flag.String(discovery.ContractWriteFlag, "",
        "Write .well-known/mcp.json and exit (for CI)")
    jwksURL := flag.String("jwks-url", "",
        "JWKS endpoint URL for JWT validation (required in production)")
    port := flag.String("port", "8080", "Listen port")
    flag.Parse()

    logger := slog.Default()

    // Budget policy: reject calls that exceed per-session token limits.
    budgetPolicy := finops.NewBudgetPolicy(finops.BudgetPolicyConfig{
        MaxSessionTokens: 100_000,
        MaxToolTokens:    10_000,
    })

    // Rate limiter: cap each tool at 30 calls/second.
    rl := resilience.NewRateLimiter(resilience.RateLimiterConfig{
        Rate:  30,
        Burst: 5,
    })

    // Circuit breaker: open after 5 consecutive failures.
    cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
        FailureThreshold: 5,
        ResetTimeout:     30 * time.Second,
    })

    reg := registry.NewToolRegistry(registry.Config{
        DefaultTimeout: 30 * time.Second,
        Middleware: []registry.Middleware{
            logging.Middleware(logger),
            finops.BudgetMiddleware(budgetPolicy),
            resilience.RateLimitMiddleware(rl),
            resilience.CircuitBreakerMiddleware(cb),
            truncate.Middleware(truncate.Config{MaxBytes: 64 * 1024}),
        },
    })

    // Register domain module (deferred for large toolsets).
    reg.RegisterDeferredModule(&module.Module{}, module.DeferredSet)

    // Discovery front door: always eager.
    reg.RegisterModule(toolindex.NewToolIndexModule("my_server", reg))

    // Server card for MCP directory and --contract-write CI generation.
    cardCfg := discovery.MetadataConfig{
        Name:         "io.github.my-org.my-mcp-server",
        Description:  "Production MCP server for my-org",
        Version:      "1.0.0",
        Organization: "my-org",
        Repository:   "https://github.com/my-org/my-mcp-server",
        License:      "MIT",
        Categories:   []string{"developer-tools"},
        Transports:   []discovery.TransportInfo{{Type: "streamable-http", URL: "http://localhost:8080/mcp"}},
        Install:      &discovery.InstallInfo{Go: "go install github.com/my-org/my-mcp-server@latest"},
        Tools:        reg,
    }
    if err := discovery.HandleContractWrite(*contractWrite, cardCfg); err != nil {
        if errors.Is(err, discovery.ErrContractWritten) {
            log.Printf("wrote server card to %s", *contractWrite)
            return
        }
        log.Fatal(err)
    }

    checker := health.NewChecker(health.WithToolCount(reg.ToolCount))

    mcpServer := server.NewMCPServer(
        "my-mcp-server", "1.0.0",
        server.WithToolCapabilities(true),
        server.WithRecovery(),
    )

    // Optional: JWT auth middleware. Skip JWKS URL for local dev.
    if *jwksURL != "" {
        validator, err := auth.NewJWTValidator(auth.JWTConfig{JWKSURL: *jwksURL})
        if err != nil {
            log.Fatal(err)
        }
        _ = validator // wire into HTTP middleware or sampling context
    }

    reg.RegisterWithServer(mcpServer)

    httpTransport := server.NewStreamableHTTPServer(mcpServer,
        server.WithEndpointPath("/mcp"),
        server.WithStateLess(true),
    )

    mux := http.NewServeMux()
    mux.Handle("/mcp", httpTransport)
    mux.Handle("/health", health.Handler(checker))
    mux.Handle("/ready", health.Handler(checker))
    mux.Handle("/.well-known/mcp.json", discovery.ServerCardHandler(cardCfg))

    httpServer := &http.Server{
        Addr:         ":" + *port,
        Handler:      mux,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 60 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    lm := lifecycle.New(lifecycle.Config{
        DrainTimeout: 15 * time.Second,
        OnHealthy: func() {
            checker.SetStatus("healthy")
            log.Printf("my-mcp-server: listening on :%s/mcp", *port)
        },
        OnDraining: func() {
            checker.SetStatus("draining")
        },
    })
    lm.OnShutdown(func(ctx context.Context) error {
        return httpServer.Shutdown(ctx)
    })

    if err := lm.Run(context.Background(), func(ctx context.Context) error {
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            return err
        }
        return nil
    }); err != nil {
        log.Fatal(err)
    }
}
```

See `examples/http/main.go` and `examples/full/main.go` for reference implementations of this pattern.

### Registering with Claude Code

```json
{
  "mcpServers": {
    "my-server": {
      "type": "http",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

For remote deployments:

```json
{
  "mcpServers": {
    "my-server": {
      "type": "http",
      "url": "https://mcp.my-org.com/mcp",
      "headers": {
        "Authorization": "Bearer ${MY_SERVER_TOKEN}"
      }
    }
  }
}
```

### CI: generate server card without starting the server

```bash
go run ./cmd/my-mcp-server --contract-write .well-known/mcp.json
```

The `discovery.HandleContractWrite` helper checks the flag, writes the JSON, and returns `discovery.ErrContractWritten` to signal a clean exit. Wire it into your CI pipeline to keep the published server card in sync with registered tools.

### Pros

- Full middleware stack: auth, rate limiting, circuit breakers, finops, truncation
- StreamableHTTP transport supports remote clients, load balancers, and proxies
- Health endpoints (`/health`, `/ready`, `/live`) integrate with k8s probes and uptime monitors
- Server card (`/.well-known/mcp.json`) enables MCP registry discovery
- Graceful drain on SIGTERM ensures in-flight tool calls complete before shutdown
- Separate module path means `go install github.com/my-org/my-mcp-server@latest` installs directly

### Cons

- Higher initial complexity than patterns A or B
- Requires network access from the MCP host (not always available in air-gapped or sandboxed environments)
- Rate limiters and circuit breakers are in-process — not distributed; use an API gateway for multi-instance deployments
- TLS termination is the operator's responsibility (use a reverse proxy or cert-manager)

### When to choose Pattern C

- Shared team infrastructure: every engineer's Claude Code points at the same server
- SaaS MCP servers: external users, billing, per-tenant rate limits
- CI/CD integrations: the server runs on your infra, clients connect remotely
- Toolsets requiring auth (OAuth 2.1, JWKS, workload identity via GCP/AWS)
- Servers that need independent versioning and release cadences from the main application

---

## Decision Matrix

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                       MCP Deployment Pattern Decision Matrix                        │
├─────────────────────────────┬──────────────────┬──────────────────┬─────────────────┤
│ Factor                      │ A: .mcp.json     │ B: Discovery     │ C: Sidecar      │
│                             │ Inline           │ Contract Layer   │ Package         │
├─────────────────────────────┼──────────────────┼──────────────────┼─────────────────┤
│ Tool count                  │ < 20             │ 20 – 500+        │ Any             │
│ Context budget concern      │ Low              │ High             │ Any             │
│ Auth required               │ No               │ No               │ Yes             │
│ Rate limiting required      │ No               │ No               │ Yes             │
│ Deployment environment      │ Local only       │ Local or shared  │ Remote / shared │
│ Multi-user / team shared    │ No               │ Maybe            │ Yes             │
│ Build step required         │ Optional         │ Yes (Go binary)  │ Yes (Go binary) │
│ Infrastructure required     │ None             │ None             │ HTTP server     │
│ Cold-start latency          │ go run: ~1s      │ go run: ~1s      │ Always-on: ~0ms │
│ Health probes               │ No               │ No               │ Yes             │
│ MCP registry publishable    │ Manual           │ Manual           │ Yes (server card│
│                             │                  │                  │ + --contract-   │
│                             │                  │                  │ write)          │
│ Graceful shutdown           │ No               │ No               │ Yes (lifecycle) │
│ Circuit breakers            │ Optional         │ Optional         │ Yes (resilience)│
│ Best for                    │ Personal tools,  │ Large toolsets,  │ Team infra,     │
│                             │ prototyping      │ context budgets  │ SaaS, auth      │
└─────────────────────────────┴──────────────────┴──────────────────┴─────────────────┘
```

### Quick-pick flowchart

```
Is your toolset shared across multiple engineers or external users?
    YES → Pattern C (Sidecar Package)
    NO  → continue

Does your toolset have 20+ tools or does context budget matter?
    YES → Pattern B (Discovery Contract Layer)
    NO  → continue

Are you prototyping, distributing a binary, or building a personal tool?
    YES → Pattern A (.mcp.json Inline)
```

---

## Migrating Between Patterns

### A → B (adding deferred loading to a growing toolset)

1. Keep the existing binary entry point.
2. Add `toolindex` as a dependency.
3. Replace `reg.RegisterModule(&MyModule{})` with `reg.RegisterDeferredModule(&MyModule{}, deferredSet)` for large modules.
4. Add `reg.RegisterModule(toolindex.NewToolIndexModule("my_server", reg))` to expose the discovery front door.
5. Update the `.mcp.json` to point to the same binary — no config change needed.

```diff
-reg.RegisterModule(&analytics.Module{})
+reg.RegisterDeferredModule(&analytics.Module{}, map[string]bool{
+    "analytics_query":  true,
+    "analytics_export": true,
+})
+reg.RegisterModule(toolindex.NewToolIndexModule("my_server", reg))
```

### B → C (adding production infrastructure)

1. Create a separate module (`go mod init github.com/my-org/my-mcp-server`).
2. Extract the tool modules into `pkg/` sub-packages.
3. Add the middleware stack: `logging`, `finops`, `resilience`, `truncate`.
4. Switch from `registry.ServeStdio` to `server.NewStreamableHTTPServer` with a lifecycle manager.
5. Wire in `discovery.MetadataConfig` and `health.NewChecker`.
6. Update `.mcp.json` to use `"type": "http"` and point at the new HTTP endpoint.

The tool module code does not change — only the main.go wiring changes.

### A → C (direct jump for personal → team promotion)

Follow the B → C steps above. The tool implementation (`TypedHandler` definitions, `ToolModule.Tools()` methods) is portable without modification. Only the registry wiring and transport configuration change.

---

## Package Reference

| Pattern | Required packages | Optional packages |
|---------|-------------------|-------------------|
| A | `registry`, `handler` | `resilience`, `logging` |
| B | `registry`, `handler`, `toolindex` | `skills`, `resilience` |
| C | `registry`, `handler`, `toolindex`, `discovery`, `health`, `lifecycle` | `auth`, `finops`, `resilience`, `observability`, `middleware/truncate`, `security` |

All packages are independently importable. You never need the full stack to get started.

---

## See Also

- `examples/minimal/` — Pattern A reference implementation
- `examples/http/` — Pattern C reference implementation (StreamableHTTP, health, server card)
- `examples/full/` — Pattern C with the full middleware stack
- `docs/QUICKSTART.md` — 5-stage progressive tutorial from hello world to production
- `toolindex/` — discovery front-door implementation (Pattern B)
- `skills/` — trigger-based lazy tool activation (Pattern B extension)
- `discovery/` — server card and MCP registry publishing (Pattern C)
- `lifecycle/` — graceful shutdown (Pattern C)
