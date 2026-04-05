# Contributing to mcpkit

Thank you for your interest in contributing to mcpkit, a Go toolkit for building
production-grade MCP (Model Context Protocol) servers. Whether you are fixing a
bug, adding a feature, improving documentation, or writing tests, your
contribution is welcome.

**Before you start:**

- Browse [GitHub Issues](https://github.com/hairglasses-studio/mcpkit/issues) for
  existing work and discussion.
- Use [GitHub Discussions](https://github.com/hairglasses-studio/mcpkit/discussions)
  for feature ideas and questions.
- Read the [Code of Conduct](CODE_OF_CONDUCT.md).

## Development Setup

### Prerequisites

- **Go 1.26.1** or later
- **git**
- **make** (for `pipeline.mk` targets)

### Clone and Build

```bash
git clone https://github.com/hairglasses-studio/mcpkit.git
cd mcpkit
make build      # or: go build ./...
```

### Run Tests

```bash
make test       # or: go test ./... -count=1
```

### Run Tests with Race Detection

```bash
go test ./... -count=1 -race
```

### Lint

```bash
go vet ./...
```

### Full Pipeline

```bash
make check      # build + vet + test
```

mcpkit uses a shared `pipeline.mk` Makefile include that provides standardized
targets across all hairglasses-studio repos: `build`, `vet`, `test`, `lint`,
`bench`, `coverage`. You can run any of these with `make <target>`.

## Pull Request Workflow

1. **Fork** the repository and clone your fork.
2. **Create a branch** from `main` using the naming convention below.
3. **Make your changes** with tests.
4. **Run the full pipeline locally:**
   ```bash
   make build && go vet ./... && go test ./... -count=1 -race
   ```
5. **Commit** with conventional commit messages (see below).
6. **Push** to your fork and open a PR against `main`.
7. **Fill out the PR description** covering what changed, why, and how you tested it.
8. **Wait for CI** and maintainer review.

Keep PRs focused. One logical change per PR is easier to review than a combined
refactor-plus-feature.

### Branch Naming

Use `type/short-description`:

| Prefix | Use for |
|--------|---------|
| `feat/` | New features |
| `fix/` | Bug fixes |
| `docs/` | Documentation changes |
| `test/` | Test additions or improvements |
| `refactor/` | Code restructuring without behavior changes |
| `chore/` | Dependency updates, CI, tooling |

Examples: `feat/streaming-transport`, `fix/nil-context-middleware`, `docs/handler-examples`

### Commit Messages

Use conventional-style prefixes:

```
feat: add streaming transport support
fix: handle nil context in middleware chain
docs: add ToolModule authoring example
test: add table-driven tests for registry
refactor: simplify middleware chain resolution
chore: update go.mod dependencies
```

### Review Timeline

| Priority | First response |
|----------|---------------|
| P0 (security/production) | 4 hours |
| P1 (blocking bug) | 24 hours |
| P2 (feature/enhancement) | 3 days |
| P3 (docs/chore) | 7 days |

## Architecture Overview

mcpkit is organized into 35+ packages across four dependency layers. The core
packages that most contributors will interact with are:

### `handler`

The type-safe tool authoring layer. Provides `TypedHandler[In, Out]` which
auto-generates JSON Schema from Go struct tags and populates `structuredContent`.
Also provides param extraction helpers (`GetStringParam`, `GetIntParam`), result
builders (`TextResult`, `JSONResult`, `ErrorResult`), and the coded error result
pattern. All tool handlers flow through this package.

### `registry`

The central tool registry and middleware chain. `ToolRegistry` holds tool
definitions, applies middleware, and wires everything to an MCP server.
Defines the canonical `Middleware` type signature, `ToolDefinition`,
`ToolHandlerFunc`, and SDK compatibility aliases. Also handles tool integrity
verification via SHA-256 fingerprinting.

### `mcptest`

Testing infrastructure for MCP servers. `mcptest.NewServer(t, tools...)` spins
up an in-process test server connected via stdio. Provides assertion helpers,
session record/replay, golden file snapshots, and benchmark helpers. All
integration tests should use this package.

### `middleware` and `resilience`

Cross-cutting middleware: rate limiting, circuit breakers, caching, logging,
and observability. Middleware follows the standard signature defined in
`registry.Middleware`. The `resilience` package provides production-grade
circuit breakers, rate limiters, and cache generics.

### `gateway`

Multi-server aggregation with namespaced tool routing. Routes tool calls to
upstream MCP servers with per-upstream resilience policies (circuit breakers,
rate limits, timeouts). Supports dynamic upstream registration and
session affinity.

### Dependency Layers

```
Layer 1 (no internal deps):  registry, health, sanitize, secrets, client, transport
Layer 2 (depend on L1):      handler, resilience, mcptest, auth, resources, prompts, ...
Layer 3 (depend on L2):      security, gateway, ralph, skills, rdcycle
Layer 4 (depend on L3):      orchestrator, handoff, workflow, bootstrap
```

Packages must only depend on their own layer or lower. Never add an upward
dependency.

## Code Conventions

### Formatting and Linting

All code must pass these checks before submission:

```bash
gofmt -l .              # Must produce no output (all files formatted)
go vet ./...            # Must produce no warnings
golangci-lint run ./... # If installed; covers staticcheck, errcheck, etc.
```

Use `gofmt` (or `goimports`) to format code. CI runs `go vet` on every PR.
If you have `golangci-lint` installed, run it locally to catch issues early.

### Error Handling

Tool handlers must **always** return `(*mcp.CallToolResult, nil)`. Never return
`(nil, error)` from a handler. Use `handler.CodedErrorResult` for error
responses:

```go
// Correct: return a coded error result
return handler.CodedErrorResult(handler.ErrInvalidParam, err), nil

// Wrong: never return nil result with an error
return nil, err  // DO NOT DO THIS
```

Never use naked panics. If something is genuinely unrecoverable, log it and
return an error result.

### Thread Safety

Protect shared state with `sync.RWMutex`. Use `RLock` for reads and `Lock` for
writes:

```go
type Module struct {
    mu      sync.RWMutex
    entries map[string]*Entry
}

// RLock for reads
func (m *Module) Get(key string) *Entry {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.entries[key]
}

// Lock for writes
func (m *Module) Set(key string, e *Entry) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.entries[key] = e
}
```

### Middleware Signature

All middleware must follow this exact signature:

```go
func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc
```

This is defined as `registry.Middleware` in the `registry` package.

### General Style

- Format with `gofmt` (or `goimports`).
- Pass `go vet ./...` with no warnings.
- No exported globals -- use `init()` for registration only.
- Prefer returning concrete types over interfaces.
- Error messages: lowercase, no trailing punctuation.
- Context: always accept and propagate `context.Context`.
- Imports: stdlib first, then external, then internal (blank line separated):

```go
import (
    "context"
    "fmt"

    "github.com/mark3labs/mcp-go/server"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)
```

## Testing Requirements

### All PRs Must

- Include tests for new functionality.
- Pass `go test ./... -count=1` with zero failures.
- Pass `go test ./... -count=1 -race` with zero race conditions.
- Maintain or improve test coverage (target: >90% per package).

### Table-Driven Tests (Required Pattern)

Use table-driven tests with `t.Run` subtests for all new test functions:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid input", input: "hello", want: "HELLO"},
        {name: "empty input", input: "", want: ""},
        {name: "invalid utf8", input: "\xff", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

Use `mcptest.NewServer()` for integration tests that exercise tools through the
MCP protocol:

```go
func TestToolIntegration(t *testing.T) {
    srv := mcptest.NewServer(t, myModule.Tools()...)
    defer srv.Close()

    result, err := srv.CallTool("my_tool", map[string]any{
        "param": "value",
    })
    if err != nil {
        t.Fatal(err)
    }
    // Assert on result...
}
```

### Unit Tests

- Use stdlib `testing` package (no testify, no gomega).
- Name test files `*_test.go` in the same package.
- Test error paths, not just happy paths.
- Each package's tests must pass in isolation: `go test ./<package>/ -count=1`

## Writing a New ToolModule

mcpkit organizes tools into **modules** -- groups of related tools that share a
name and description. Here is how to create one from scratch.

### Step 1: Create the Module Struct

Create a new package (or add to an existing one) and define a struct that
implements the `registry.ToolModule` interface:

```go
package mytools

import (
    "github.com/hairglasses-studio/mcpkit/registry"
)

// Module implements registry.ToolModule for the mytools domain.
type Module struct{}

func (m *Module) Name() string        { return "mytools" }
func (m *Module) Description() string { return "My custom tools" }
```

### Step 2: Define Input/Output Types

Use Go structs with `json` and `jsonschema` tags. mcpkit generates JSON Schema
from these tags automatically:

```go
type LookupInput struct {
    ID     string `json:"id" jsonschema:"required,description=The item ID to look up"`
    Format string `json:"format,omitempty" jsonschema:"description=Output format (json or text),enum=json,enum=text"`
}

type LookupOutput struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

### Step 3: Implement Tools()

Return a slice of `registry.ToolDefinition`. Use `handler.TypedHandler` for
type-safe handlers that auto-generate schemas:

```go
func (m *Module) Tools() []registry.ToolDefinition {
    return []registry.ToolDefinition{
        handler.TypedHandler[LookupInput, LookupOutput](
            "mytools_lookup",
            "Look up an item by ID.",
            m.handleLookup,
        ),
    }
}
```

### Step 4: Write the Handler

Handlers receive a typed input and return a typed output. Errors are returned as
Go errors and mcpkit translates them into MCP error results:

```go
func (m *Module) handleLookup(ctx context.Context, input LookupInput) (LookupOutput, error) {
    // Validate
    if input.ID == "" {
        return LookupOutput{}, fmt.Errorf("id is required")
    }

    // Do work...
    item, err := lookupItem(ctx, input.ID)
    if err != nil {
        return LookupOutput{}, err
    }

    return LookupOutput{
        Name:   item.Name,
        Status: item.Status,
    }, nil
}
```

If you are writing a raw handler (not using `TypedHandler`), always follow the
handler contract:

```go
func handleMyTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    name, err := handler.RequiredParam[string](req, "name")
    if err != nil {
        return handler.CodedErrorResult(handler.ErrInvalidParam, err), nil
    }

    // Do work...

    // ALWAYS return (result, nil) -- never (nil, error)
    return handler.TextResult("Success: " + name), nil
}
```

### Step 5: Register the Module

Register your module with a `ToolRegistry` and wire it to an MCP server:

```go
func main() {
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&mytools.Module{})

    s := registry.NewMCPServer("my-server", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

### Step 6: Write Tests

Add both unit tests and an integration test:

```go
// mytools_test.go
func TestModule_Tools(t *testing.T) {
    m := &Module{}
    tools := m.Tools()
    if len(tools) == 0 {
        t.Fatal("expected at least one tool")
    }
    if tools[0].Tool.Name != "mytools_lookup" {
        t.Errorf("got tool name %q, want %q", tools[0].Tool.Name, "mytools_lookup")
    }
}

func TestLookupIntegration(t *testing.T) {
    m := &Module{}
    srv := mcptest.NewServer(t, m.Tools()...)
    defer srv.Close()

    result, err := srv.CallTool("mytools_lookup", map[string]any{
        "id": "abc-123",
    })
    if err != nil {
        t.Fatal(err)
    }
    // Assert on result content...
}
```

### Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Module name | `snake_case` | `system_info` |
| Tool name | `module_action` | `system_info_get`, `process_list` |
| Package name | Short, matches concept | `sysinfo`, `proc` |

## Writing New Middleware

Middleware wraps tool handlers with cross-cutting behavior (logging, auth,
metrics, rate limiting). All middleware uses the same signature defined as
`registry.Middleware`.

### Step 1: Implement the Middleware Function

A middleware factory returns a `registry.Middleware`. The middleware receives
the tool name, its definition, and the next handler in the chain. It returns a
new handler that wraps `next`:

```go
package mymiddleware

import (
    "context"
    "log/slog"
    "time"

    "github.com/hairglasses-studio/mcpkit/registry"
)

// TimingMiddleware logs the execution time of each tool call.
func TimingMiddleware(logger *slog.Logger) registry.Middleware {
    return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
        return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
            start := time.Now()

            // Call the next handler in the chain
            result, err := next(ctx, request)

            logger.InfoContext(ctx, "tool timing",
                "tool", name,
                "duration_ms", time.Since(start).Milliseconds(),
            )

            return result, err
        }
    }
}
```

### Step 2: Apply the Middleware

Register middleware with the tool registry. Middleware is applied in order --
the first registered middleware is the outermost wrapper:

```go
reg := registry.NewToolRegistry()
reg.AddMiddleware(mymiddleware.TimingMiddleware(logger))
reg.AddMiddleware(logging.Middleware(logger))
```

### Step 3: Use Tool Definition Metadata

Middleware receives `td registry.ToolDefinition`, which contains the tool's
category, annotations, and other metadata. Use this to make middleware
context-aware:

```go
func CategoryFilterMiddleware(allowedCategory string) registry.Middleware {
    return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
        if td.Category != allowedCategory {
            return next // pass through unchanged
        }
        return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
            // Only applied to tools in the allowed category
            return next(ctx, request)
        }
    }
}
```

### Step 4: Write Tests

Test middleware by creating a mock next handler and verifying the wrapper
behavior:

```go
func TestTimingMiddleware(t *testing.T) {
    var logged bool
    logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

    mw := TimingMiddleware(logger)
    td := registry.ToolDefinition{
        Tool: registry.Tool{Name: "test_tool"},
    }

    next := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
        return registry.MakeTextResult("ok"), nil
    }

    wrapped := mw("test_tool", td, next)
    result, err := wrapped(context.Background(), registry.CallToolRequest{})
    if err != nil {
        t.Fatal(err)
    }
    if result == nil {
        t.Fatal("expected non-nil result")
    }
}
```

### Middleware Guidelines

- Always call `next(ctx, request)` unless you are intentionally short-circuiting
  (e.g., authorization failure).
- Never modify the incoming `request` unless your middleware is explicitly a
  request transformer.
- Return `(result, err)` from `next` unchanged unless your middleware is a
  response transformer.
- Use `registry.IsResultError(result)` to check if the downstream handler
  returned an error result.
- Keep middleware stateless when possible. If state is needed, protect it with
  `sync.RWMutex`.

## Reporting Bugs

Open a [GitHub Issue](https://github.com/hairglasses-studio/mcpkit/issues) with:

- Go version and OS
- Minimal reproduction steps
- Expected vs. actual behavior
- Relevant error output or logs

## Suggesting Features

Open a [GitHub Issue](https://github.com/hairglasses-studio/mcpkit/issues) with
the `type/feature` label. Describe the use case, not just the solution. If the
feature involves API changes, include a short code sketch showing how it would
be called.

## Issue Labels

We use a unified label taxonomy:

| Label | Meaning |
|-------|---------|
| `type/bug` | Bug report |
| `type/feature` | New feature |
| `type/enhancement` | Improvement to existing feature |
| `type/docs` | Documentation |
| `type/chore` | Maintenance, CI, dependencies |
| `priority/P0-critical` | Security or production issue |
| `priority/P1-high` | Blocking bug |
| `priority/P2-medium` | Feature or enhancement |
| `priority/P3-low` | Docs, chore |
| `good-first-issue` | Newcomer-friendly |
| `help-wanted` | Community help needed |

## Getting Help

- **GitHub Discussions:** Feature ideas and Q&A
- **CLAUDE.md** in the repo root: Architecture overview, package map, and key patterns
- **README.md**: Quick start guide and package reference

## License

mcpkit is MIT licensed (Copyright 2024-2026 hairglasses-studio). By
contributing, you agree that your contributions will be licensed under the same
[MIT License](LICENSE).
