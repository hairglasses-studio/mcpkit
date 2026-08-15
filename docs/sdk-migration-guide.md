# mcp-go to Official Go SDK Migration Guide

This guide covers moving an mcpkit-based server from direct `mcp-go` coupling toward the official `github.com/modelcontextprotocol/go-sdk` build path. mcpkit still defaults to `github.com/mark3labs/mcp-go`; the `official_sdk` build tag is a migration lane that lets tool modules move first while SDK-specific server edges move later.

For the historical v2 readiness check, see [go-sdk v2.0 Compatibility Assessment](sdk-v2-compat-assessment.md). mcpkit is pinned to `github.com/modelcontextprotocol/go-sdk v1.7.0`, whose 2026-07-28 protocol path adds stateless `server/discover` negotiation and request-routing headers.

## Consumer pin status (2026-05-10, post-v0.6.0)

Fleet consumers of mcpkit pin to the following versions (audit from `~/hairglasses-studio/*/go.mod`):

| Repo | Pin | Migration status |
|---|---|---|
| mapitall | `v0.3.1-0.20260405143724-9e0ed2c34989` (with `replace` to local) | Local replace — auto-tracks main |
| github-runner-mcp | `v0.5.1` (with `replace` to local) | Local replace — auto-tracks main |
| docs | `v0.5.1` | Bump to v0.6.0 recommended (additive only; no migration needed) |
| shielddd | `v0.5.2` | Closest to current; bump to v0.6.0 recommended |
| secretstudios-mcp | `v0.5.1` (with `replace` to local) | Local replace — auto-tracks main |
| shopp | `v0.5.1` | Bump to v0.6.0 recommended |

No fleet consumer is on the dual-SDK build tag (`-tags official_sdk`) by default; mcpkit's `mark3labs/mcp-go` path remains the production runtime. For consumers that *do* want to test the official-SDK path, `make build-official` + `make test-official` are the canonical entrypoints (see Current Support below).

## Current Support

Use these authoritative local gates before and after every migration slice.
GitHub Actions is deprecated and is not a substitute for these checks:

```bash
make check
make build-official
make test-official
# or run the combined gate:
make check-dual
```

`make build-official` currently compiles the supported official-SDK package set:

```text
./registry ./handler ./mcptest ./transport ./session ./health ./sampling ./resources ./prompts ./feedback ./testing/conformance ./middleware/correlation ./middleware/gate ./observability ./discovery ./sanitize ./resilience ./gateway ./a2a ./bridge/a2a
```

`make test-official` intentionally runs a narrower test set where official-SDK fixtures are complete:

```text
./registry ./handler ./mcptest ./transport ./session ./health ./sampling ./resources ./prompts ./feedback ./testing/conformance ./middleware/correlation ./middleware/gate ./observability ./discovery ./sanitize ./resilience ./gateway ./a2a ./bridge/a2a
```

Do not change that test scope to `./...` until every package fixture is SDK-neutral.

## Migration Rule

Application modules should import mcpkit APIs, not either SDK directly:

```go
import (
    "context"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)
```

Keep direct SDK imports in one of these places only:

- mcpkit compatibility files such as `registry/compat.go` and `registry/compat_official.go`
- server edge files guarded by `//go:build !official_sdk` or `//go:build official_sdk`
- tests that are explicitly scoped to one SDK path

## API Differences

| Surface | mcp-go default | official go-sdk | mcpkit migration path |
|---|---|---|---|
| Server creation | `server.NewMCPServer(name, version, opts...)` | `mcp.NewServer(&mcp.Implementation{...}, opts)` | `registry.NewMCPServer(name, version)` |
| Stdio serving | `server.ServeStdio(s)` | `server.Run(ctx, &mcp.StdioTransport{})` or `Connect` | `registry.ServeStdio(s)` |
| Tool registration | `s.AddTool(tool, handler)` | `s.AddTool(&tool, handler)` or generic `mcp.AddTool` | `registry.AddToolToServer(s, td.Tool, td.Handler)` |
| Tool schema | concrete `mcp.ToolInputSchema` | `any` holding a JSON schema value | use `handler.TypedHandler` or build-tagged schema helpers |
| Tool request args | `req.Params.Arguments` is map-like `any` | `req.Params.Arguments` is raw JSON on server handlers | `registry.ExtractArguments(req)`, `registry.NewCallToolRequest(...)` |
| Text content | value `mcp.TextContent` | pointer `*mcp.TextContent` | `registry.MakeTextContent`, `registry.ExtractTextContent` |
| Resource contents | slice of content interface values | slice of `*mcp.ResourceContents` | `registry.ExtractResourceText` and compatibility helpers |
| Prompts | value message/content types | pointer content/message shapes | keep prompt handlers behind package compatibility wrappers |
| Tasks | supported in mcp-go types | not yet equivalent in official SDK path | keep task features behind registry helpers |

## Step-by-Step

1. Update to an mcpkit version with the dual-SDK gates:

   ```bash
   go get github.com/hairglasses-studio/mcpkit@latest
   go mod tidy
   ```

2. Replace direct `mcp-go` imports in tool modules with `registry` and `handler`.

   Before:

   ```go
   func (m *Module) Tools() []registry.ToolDefinition {
       return []registry.ToolDefinition{
           {
               Tool: mcp.NewTool("echo", mcp.WithString("message")),
               Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
                   msg := req.GetString("message", "")
                   return mcp.NewToolResultText(msg), nil
               },
           },
       }
   }
   ```

   After:

   ```go
   type echoInput struct {
       Message string `json:"message" jsonschema:"required,description=Message to echo"`
   }

   type echoOutput struct {
       Message string `json:"message"`
   }

   func (m *Module) Tools() []registry.ToolDefinition {
       return []registry.ToolDefinition{
           handler.TypedHandler[echoInput, echoOutput](
               "echo",
               "Echo a message",
               func(ctx context.Context, in echoInput) (echoOutput, error) {
                   return echoOutput{Message: in.Message}, nil
               },
           ),
       }
   }
   ```

3. For untyped handlers, route all request and result access through mcpkit helpers:

   ```go
   Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
       msg := handler.GetStringParam(req, "message")
       return registry.MakeTextResult(msg), nil
   }
   ```

4. Move unavoidable SDK-specific code into paired files:

   ```go
   //go:build !official_sdk
   ```

   ```go
   //go:build official_sdk
   ```

   Keep the exported package API identical across both files. Tests should use the same pattern for SDK-specific request construction or schema literals.

5. Add both build paths to the repository-local CI contract:

   ```bash
   make test
   make build-official
   make test-official
   ```

6. Expand official coverage package by package. A package is ready to enter `OFFICIAL_SDK_TEST_PACKAGES` only when:

   - it has no unguarded direct `mcp-go` imports in tests
   - its handlers use `registry.*` types or build-tagged adapters
   - `go test -tags official_sdk ./that/package -count=1` passes
   - default `go test ./that/package -count=1` still passes

## Resource and Prompt Packages

`resources` and `prompts` already build under `official_sdk`, but much of their test suite still uses `mcp-go` fixtures directly. Treat them as build-supported and test-migration pending until their test helpers are split into SDK-neutral fixtures.

## Compatibility Checklist

- Tool modules import `registry` and `handler`, not SDK packages.
- Schema construction is typed-handler generated or hidden behind build-tagged helpers.
- Argument reads use `registry.ExtractArguments` or `handler.Get*Param`; test fixtures and adapters use `registry.NewCallToolRequest` or `registry.SetCallToolArguments`.
- Content creation uses `registry.MakeTextContent`, `registry.MakeTextResult`, or `handler.TextResult`.
- Server registration uses `registry.NewMCPServer`, `registry.AddToolToServer`, and `registry.ServeStdio`.
- Local CI runs the default SDK path and the official-SDK package set.
- Roadmap items that depend on latest upstream versions are verified separately before being marked complete.
