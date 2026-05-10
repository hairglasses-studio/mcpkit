# Build Your First MCP Server Tutorial Outline

This outline turns `docs/QUICKSTART.md` into a publishable tutorial series. Each part should end with runnable code, a concrete validation command, and one visible MCP client interaction.

## Series Shape

| Part | Working title | Outcome |
|---|---|---|
| 1 | Create a typed stdio server | One `echo` tool runs in an MCP client |
| 2 | Model useful inputs and errors | Parameters, enums, optional fields, and typed error handling are visible in the client |
| 3 | Organize tools into modules | Multiple tools are registered through `ToolModule` and package boundaries |
| 4 | Add middleware and safety gates | Logging, truncation, rate limits, and write confirmation are wired into calls |
| 5 | Add resources and prompts | A server exposes tools, resources, and reusable prompt templates |
| 6 | Test and benchmark the server | `mcptest` covers calls, schemas, golden output, and benchmark baselines |
| 7 | Ship over HTTP | StreamableHTTP, health endpoints, server card, and lifecycle drain are production-ready |
| 8 | Containerize and operate | Docker Compose, health checks, and deployment smoke tests run end to end |

## Part 1: Create a Typed Stdio Server

Goal: get from an empty Go module to a working local MCP server.

Code milestone:
- initialize `go.mod`
- define `EchoInput` and `EchoOutput`
- register `handler.TypedHandler`
- serve with `registry.ServeStdio`

Validation:

```bash
go run main.go
npx @modelcontextprotocol/inspector go run main.go
```

Callout: explain that struct tags become JSON Schema and typed outputs become `structuredContent`.

## Part 2: Model Useful Inputs and Errors

Goal: teach enough schema design for real tools.

Code milestone:
- required fields
- optional fields with `omitempty`
- enum-like schema tags
- validation errors from handler logic

Validation:

```bash
go test ./... -count=1
```

Client interaction: call the same tool with valid input, missing input, and wrong types.

## Part 3: Organize Tools Into Modules

Goal: move from one-file demos to maintainable packages.

Code milestone:
- create a domain package
- implement `Name`, `Description`, and `Tools`
- register the module through `ToolRegistry`
- add category, tags, search terms, complexity, and write metadata

Validation:

```bash
go test ./... -count=1
```

Reference example: `examples/minimal` for the smallest server shape and `examples/full` for a richer module.

## Part 4: Add Middleware and Safety Gates

Goal: show the production value of mcpkit beyond raw SDK calls.

Code milestone:
- global logging middleware
- response truncation middleware
- resilience middleware with timeout or circuit-breaker grouping
- bounded-write confirmation for mutating tools

Validation:

```bash
go test ./... -count=1
```

Reference examples: `examples/bounded-write`, `examples/truncate-demo`, and `examples/full`.

## Part 5: Add Resources and Prompts

Goal: cover the full MCP authoring surface.

Code milestone:
- expose a static documentation resource
- expose a templated resource
- expose a prompt with required and optional arguments
- register resources and prompts alongside tools

Validation:

```bash
go test ./... -count=1
```

Reference example: `examples/full`.

## Part 6: Test and Benchmark the Server

Goal: make tool behavior reproducible before deployment.

Code milestone:
- call tools with `mcptest`
- assert schema fields
- add golden output snapshots for stable responses
- capture benchmark output and parse it with `mcptest.ParseBenchmarkOutput`

Validation:

```bash
go test ./... -count=1
go test -bench=. ./... -run '^$'
```

Reference docs: `docs/BENCHMARK.md`.

## Part 7: Ship Over HTTP

Goal: convert the local server into a service.

Code milestone:
- use StreamableHTTP on `/mcp`
- expose `/health`, `/ready`, and `/live`
- publish `/.well-known/mcp.json`
- use `lifecycle.Manager` for SIGTERM drain

Validation:

```bash
go run ./examples/http
curl -s http://localhost:8080/health
curl -s http://localhost:8080/.well-known/mcp.json
```

Reference example: `examples/http`.

## Part 8: Containerize and Operate

Goal: make the server deployable in a realistic local stack.

Code milestone:
- multi-stage Dockerfile
- non-root runtime user
- Docker Compose service health checks
- nginx or another reverse proxy in front of multiple app instances
- identity smoke endpoint for load-balancing verification

Validation:

```bash
cd examples/docker
docker compose up --build
curl -s http://localhost:8080/identity
curl -s http://localhost:8080/ready
```

Reference example: `examples/docker`.

## Production Checklist

- every episode has a branch or downloadable code state
- every command is copy-paste runnable from repo root unless stated otherwise
- every client demo shows the actual MCP result, not just server logs
- every introduced package is linked to the package map in `README.md`
- every production claim has a validation command
- final episode runs `make check-dual`

## Suggested Assets

| Asset | Purpose |
|---|---|
| repo diagram | show stdio, HTTP, health, and compose deployment paths |
| tool schema screenshot | show generated JSON Schema in an MCP client |
| middleware trace screenshot | show one tool call passing through logging/resilience/truncation |
| compose terminal capture | show health checks and two backend identities |
| final architecture diagram | show client, MCP server, resources, prompts, middleware, health, and deployment boundary |

## Publishing Order

1. Publish parts 1-3 as the beginner track.
2. Publish parts 4-6 as the reliability and testing track.
3. Publish parts 7-8 as the deployment track.
4. Add a final index page that maps each part back to `docs/QUICKSTART.md`, `examples/README.md`, and the relevant example folder.
