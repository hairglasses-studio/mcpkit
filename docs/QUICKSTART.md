# mcpkit Quick Start

A progressive, 5-stage tutorial that takes you from zero to a production-grade MCP server. Each stage builds on the previous one. All code compiles and runs.

**Prerequisites:** Go 1.22+ and a module initialized with `go mod init`.

## Installation

```bash
go get github.com/hairglasses-studio/mcpkit@latest
```

The underlying MCP protocol implementation is pulled in transitively:

```bash
go get github.com/mark3labs/mcp-go@latest
```

---

## Stage 1: Hello World (30 seconds)

Create a single-tool MCP server. This is the minimum viable server.

Create `main.go`:

```go
package main

import (
    "context"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type EchoInput struct {
    Message string `json:"message" jsonschema:"required,description=Message to echo back"`
}

type EchoOutput struct {
    Reply string `json:"reply"`
}

func main() {
    td := handler.TypedHandler[EchoInput, EchoOutput](
        "echo", "Echo a message back to the caller",
        func(ctx context.Context, in EchoInput) (EchoOutput, error) {
            return EchoOutput{Reply: "You said: " + in.Message}, nil
        },
    )

    s := registry.NewMCPServer("echo-server", "1.0.0")
    registry.AddToolToServer(s, td.Tool, td.Handler)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

**Run it:**

```bash
go run main.go
```

The server starts on stdio, waiting for JSON-RPC messages. Test it interactively with the [MCP Inspector](https://github.com/modelcontextprotocol/inspector):

```bash
npx @modelcontextprotocol/inspector go run main.go
```

**Add to Codex or Claude Code:**

Recommended Codex install:

```bash
codex mcp add echo -- go run /absolute/path/to/main.go
```

Claude compatibility via manual config:

```json
{
  "mcpServers": {
    "echo": {
      "command": "go",
      "args": ["run", "/absolute/path/to/main.go"]
    }
  }
}
```

**What you get:**
- `EchoInput` generates a JSON Schema automatically from struct tags (`jsonschema:"required,description=..."`)
- The typed output is serialized as both `content[].text` (JSON) and `structuredContent`
- The server speaks stdio JSON-RPC, the standard transport for local MCP servers

---

## Stage 2: Add Parameters (2 minutes)

Add typed parameters with validation, required vs. optional fields, and structured error codes.

Replace `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type GreetInput struct {
    Name     string `json:"name" jsonschema:"required,description=Name to greet"`
    Language string `json:"language,omitempty" jsonschema:"description=Greeting language (en or es),enum=en,enum=es"`
    Formal   bool   `json:"formal,omitempty" jsonschema:"description=Use formal greeting"`
}

type GreetOutput struct {
    Message string `json:"message"`
    Lang    string `json:"lang"`
}

func greet(_ context.Context, in GreetInput) (GreetOutput, error) {
    lang := in.Language
    if lang == "" {
        lang = "en"
    }

    if len(in.Name) > 100 {
        return GreetOutput{}, fmt.Errorf("name exceeds 100 character limit")
    }

    var msg string
    switch lang {
    case "es":
        if in.Formal {
            msg = fmt.Sprintf("Buenos dias, %s.", in.Name)
        } else {
            msg = fmt.Sprintf("Hola, %s!", in.Name)
        }
    default:
        if in.Formal {
            msg = fmt.Sprintf("Good day, %s.", in.Name)
        } else {
            msg = fmt.Sprintf("Hello, %s!", in.Name)
        }
    }

    return GreetOutput{Message: msg, Lang: lang}, nil
}

func main() {
    td := handler.TypedHandler[GreetInput, GreetOutput](
        "greet", "Greet a user by name, with language and formality options",
        greet,
    )

    s := registry.NewMCPServer("greeter", "1.0.0")
    registry.AddToolToServer(s, td.Tool, td.Handler)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

**Test it:**

```bash
npx @modelcontextprotocol/inspector go run main.go
```

Call `greet` with `{"name": "World"}` -- you get `"Hello, World!"`.
Call with `{"name": "Mundo", "language": "es", "formal": true}` -- you get `"Buenos dias, Mundo."`.
Call with `{"name": 12345}` (wrong type) -- the typed handler returns a `[INVALID_PARAM]` error automatically.

**What you get:**
- `jsonschema:"required"` marks fields as required in the generated schema; clients see the constraint, and type mismatches return `[INVALID_PARAM]` errors
- `jsonschema:"enum=en,enum=es"` constrains allowed values
- `json:",omitempty"` makes fields optional in the JSON wire format
- Returning a Go `error` from the handler produces an error result with `isError: true`

---

## Stage 3: Add Middleware (5 minutes)

Add logging and resilience middleware. Middleware wraps every tool call in the registry.

Replace `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "log/slog"
    "os"
    "time"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/middleware/truncate"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resilience"
)

// --- Types ---

type GreetInput struct {
    Name     string `json:"name" jsonschema:"required,description=Name to greet"`
    Language string `json:"language,omitempty" jsonschema:"description=Greeting language (en or es),enum=en,enum=es"`
}

type GreetOutput struct {
    Message string `json:"message"`
}

// --- Module ---

type greetModule struct{}

func (m *greetModule) Name() string        { return "greet" }
func (m *greetModule) Description() string { return "Greeting tools" }
func (m *greetModule) Tools() []registry.ToolDefinition {
    td := handler.TypedHandler[GreetInput, GreetOutput](
        "greet", "Greet a user by name",
        func(_ context.Context, in GreetInput) (GreetOutput, error) {
            lang := in.Language
            if lang == "" {
                lang = "en"
            }
            var msg string
            switch lang {
            case "es":
                msg = fmt.Sprintf("Hola, %s!", in.Name)
            default:
                msg = fmt.Sprintf("Hello, %s!", in.Name)
            }
            return GreetOutput{Message: msg}, nil
        },
    )
    td.CircuitBreakerGroup = "greet-service"
    return []registry.ToolDefinition{td}
}

// --- Logging middleware ---

func loggingMiddleware(logger *slog.Logger) registry.Middleware {
    return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
        return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
            start := time.Now()
            result, err := next(ctx, req)
            logger.Info("tool called",
                "tool", name,
                "duration", time.Since(start),
                "error", err != nil,
            )
            return result, err
        }
    }
}

// --- Main ---

func main() {
    // Always log to stderr -- stdout is reserved for MCP JSON-RPC
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

    reg := registry.NewToolRegistry(registry.Config{
        Middleware: []registry.Middleware{
            truncate.New(truncate.WithMaxBytes(4096)),
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
```

**Test it:**

```bash
npx @modelcontextprotocol/inspector go run main.go
```

Each tool call now logs a structured JSON line to stderr:

```json
{"time":"...","level":"INFO","msg":"tool called","tool":"greet","duration":"52.3us","error":false}
```

**What you get:**
- **Response truncation** via `middleware/truncate` -- caps text content at 4KB (configurable) and appends a guidance message, preventing oversized responses from flooding the model context window
- **Rate limiting** per `CircuitBreakerGroup` (default: 10 req/s, burst 20)
- **Circuit breaker** that trips after repeated failures, preventing cascading errors
- **Structured logging** to stderr (never stdout -- MCP stdio discipline)
- **Middleware signature:** `func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc` -- the same across all mcpkit packages

---

## Stage 4: Add Resources and Prompts (10 minutes)

Register a resource (a config file reader) and a prompt (a workflow template). These use separate registries that wire into the same MCP server.

Replace `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/prompts"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resources"
)

// --- Tool types ---

type GreetInput struct {
    Name string `json:"name" jsonschema:"required,description=Name to greet"`
}

type GreetOutput struct {
    Message string `json:"message"`
}

// --- Tool module ---

type greetModule struct{}

func (m *greetModule) Name() string        { return "greet" }
func (m *greetModule) Description() string { return "Greeting tools" }
func (m *greetModule) Tools() []registry.ToolDefinition {
    return []registry.ToolDefinition{
        handler.TypedHandler[GreetInput, GreetOutput](
            "greet", "Greet a user by name",
            func(_ context.Context, in GreetInput) (GreetOutput, error) {
                return GreetOutput{Message: "Hello, " + in.Name + "!"}, nil
            },
        ),
    }
}

// --- Resource module ---

type configResourceModule struct{}

func (m *configResourceModule) Name() string        { return "config" }
func (m *configResourceModule) Description() string { return "Configuration resources" }

func (m *configResourceModule) Resources() []resources.ResourceDefinition {
    return []resources.ResourceDefinition{
        {
            Resource: mcp.NewResource(
                "config://app/settings",
                "App Settings",
                mcp.WithResourceDescription("Current application settings"),
                mcp.WithMIMEType("application/json"),
            ),
            Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
                return []mcp.ResourceContents{
                    mcp.TextResourceContents{
                        URI:      "config://app/settings",
                        MIMEType: "application/json",
                        Text:     `{"log_level": "info", "max_results": 50, "language": "en"}`,
                    },
                }, nil
            },
            Category: "configuration",
        },
    }
}

func (m *configResourceModule) Templates() []resources.TemplateDefinition {
    return nil
}

// --- Prompt module ---

type workflowPromptModule struct{}

func (m *workflowPromptModule) Name() string        { return "workflows" }
func (m *workflowPromptModule) Description() string { return "Workflow prompt templates" }

func (m *workflowPromptModule) Prompts() []prompts.PromptDefinition {
    return []prompts.PromptDefinition{
        {
            Prompt: mcp.NewPrompt("greet_workflow",
                mcp.WithPromptDescription("Greet multiple users with a custom style"),
                mcp.WithArgument("names", mcp.RequiredArgument(), mcp.ArgumentDescription("Comma-separated list of names")),
                mcp.WithArgument("style", mcp.ArgumentDescription("Greeting style: casual or formal (default: casual)")),
            ),
            Handler: func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
                names := req.Params.Arguments["names"]
                style := req.Params.Arguments["style"]
                if style == "" {
                    style = "casual"
                }
                return &mcp.GetPromptResult{
                    Description: "Greet users: " + names,
                    Messages: []mcp.PromptMessage{
                        mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(
                            fmt.Sprintf(
                                "Use the greet tool to greet each of these people with a %s style: %s",
                                style, names,
                            ),
                        )),
                    },
                }, nil
            },
            Category: "workflows",
        },
    }
}

// --- Main ---

func main() {
    // Tool registry
    toolReg := registry.NewToolRegistry()
    toolReg.RegisterModule(&greetModule{})

    // Resource registry
    resReg := resources.NewResourceRegistry()
    resReg.RegisterModule(&configResourceModule{})

    // Prompt registry
    promptReg := prompts.NewPromptRegistry()
    promptReg.RegisterModule(&workflowPromptModule{})

    // Wire everything to a single MCP server
    s := server.NewMCPServer("greeter", "1.0.0",
        server.WithToolCapabilities(true),
        server.WithResourceCapabilities(false, true),
        server.WithPromptCapabilities(true),
    )
    toolReg.RegisterWithServer(s)
    resReg.RegisterWithServer(s)
    promptReg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

**Test it:**

```bash
npx @modelcontextprotocol/inspector go run main.go
```

In the Inspector:
- **Tools tab:** Call `greet` with `{"name": "World"}`
- **Resources tab:** Read `config://app/settings` to see the JSON config
- **Prompts tab:** Get `greet_workflow` with `{"names": "Alice, Bob", "style": "formal"}`

**What you get:**
- **Resources** expose data to MCP clients via URI-based read operations (`resources/read`)
- **Prompts** are reusable workflow templates that MCP clients can discover and render
- **Separate registries** (`ToolRegistry`, `ResourceRegistry`, `PromptRegistry`) each wire into the same `MCPServer`
- **Module interfaces** (`ToolModule`, `ResourceModule`, `PromptModule`) keep each concern in its own package

---

## Stage 5: Testing (15 minutes)

Use `mcptest` to write integration tests that exercise the full MCP handler chain. No Inspector needed, no network, tests run in milliseconds.

Create `main_test.go` alongside your Stage 4 `main.go`:

```go
package main

import (
    "testing"

    "github.com/hairglasses-studio/mcpkit/mcptest"
    "github.com/hairglasses-studio/mcpkit/prompts"
    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/hairglasses-studio/mcpkit/resources"
)

// helper builds a fully wired test server matching the production setup.
func newTestServer(t *testing.T) *mcptest.Client {
    t.Helper()

    toolReg := registry.NewToolRegistry()
    toolReg.RegisterModule(&greetModule{})

    resReg := resources.NewResourceRegistry()
    resReg.RegisterModule(&configResourceModule{})

    promptReg := prompts.NewPromptRegistry()
    promptReg.RegisterModule(&workflowPromptModule{})

    srv := mcptest.NewServer(t, toolReg)

    // Wire resources and prompts to the same MCP server
    resReg.RegisterWithServer(srv.MCP)
    promptReg.RegisterWithServer(srv.MCP)

    return mcptest.NewClient(t, srv)
}

func TestGreet(t *testing.T) {
    client := newTestServer(t)

    result := client.CallTool("greet", map[string]any{"name": "World"})

    mcptest.AssertNotError(t, result)
    mcptest.AssertToolResultContains(t, result, "Hello, World!")
}

func TestGreet_InvalidType(t *testing.T) {
    client := newTestServer(t)

    // Passing the wrong type triggers a deserialization error
    result, err := client.CallToolE("greet", map[string]any{"name": 12345})
    if err != nil {
        t.Fatalf("unexpected protocol error: %v", err)
    }

    mcptest.AssertError(t, result, "INVALID_PARAM")
}

func TestReadConfig(t *testing.T) {
    client := newTestServer(t)

    result := client.ReadResource("config://app/settings")

    mcptest.AssertResourceContains(t, result, `"log_level"`)
}

func TestGreetWorkflowPrompt(t *testing.T) {
    client := newTestServer(t)

    result := client.GetPrompt("greet_workflow", map[string]string{
        "names": "Alice, Bob",
        "style": "formal",
    })

    mcptest.AssertPromptMessages(t, result, 1)
    mcptest.AssertPromptContains(t, result, "formal")
    mcptest.AssertPromptContains(t, result, "Alice, Bob")
}

func TestGreet_StructuredOutput(t *testing.T) {
    client := newTestServer(t)

    result := client.CallTool("greet", map[string]any{"name": "Ada"})

    var out GreetOutput
    mcptest.AssertStructured(t, result, &out)

    if out.Message != "Hello, Ada!" {
        t.Errorf("structured message = %q, want %q", out.Message, "Hello, Ada!")
    }
}
```

**Run:**

```bash
go test -v -count=1
```

**Expected output:**

```
=== RUN   TestGreet
--- PASS: TestGreet (0.00s)
=== RUN   TestGreet_InvalidType
--- PASS: TestGreet_InvalidType (0.00s)
=== RUN   TestReadConfig
--- PASS: TestReadConfig (0.00s)
=== RUN   TestGreetWorkflowPrompt
--- PASS: TestGreetWorkflowPrompt (0.00s)
=== RUN   TestGreet_StructuredOutput
--- PASS: TestGreet_StructuredOutput (0.00s)
PASS
```

**What you get:**
- `mcptest.NewServer(t, reg)` creates a real MCP server in-process (no subprocess, no network)
- `mcptest.NewClient(t, srv)` provides `CallTool`, `ReadResource`, and `GetPrompt` methods
- Assertion helpers: `AssertNotError`, `AssertToolResultContains`, `AssertError`, `AssertStructured`, `AssertResourceContains`, `AssertPromptMessages`, `AssertPromptContains`
- Tests run in ~10ms with no external dependencies
- The test exercises the full handler chain including middleware, typed deserialization, and structured output

---

## Next Steps

| Topic | Package | Link |
|-------|---------|------|
| Typed handlers and error codes | `handler` | [pkg.go.dev/handler](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/handler) |
| Response truncation | `middleware/truncate` | [examples/truncate-demo/main.go](examples/truncate-demo/main.go) |
| Resilience (circuit breakers, rate limits) | `resilience` | [pkg.go.dev/resilience](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/resilience) |
| ToolModule interface for production servers | `registry` | [pkg.go.dev/registry](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/registry) |
| Resources and prompts | `resources`, `prompts` | [pkg.go.dev/resources](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/resources), [pkg.go.dev/prompts](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/prompts) |
| Testing infrastructure | `mcptest` | [pkg.go.dev/mcptest](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/mcptest) |
| Auth (JWT, OAuth, workload identity) | `auth` | [pkg.go.dev/auth](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/auth) |
| Multi-server gateway | `gateway` | [pkg.go.dev/gateway](https://pkg.go.dev/github.com/hairglasses-studio/mcpkit/gateway) |
| Full production example | `examples/full` | [examples/full/main.go](examples/full/main.go) |

## Debugging

Test any mcpkit server with the MCP Inspector:

```bash
npx @modelcontextprotocol/inspector ./my-server                                # compiled binary
npx @modelcontextprotocol/inspector go run ./cmd/server/                       # go run
npx @modelcontextprotocol/inspector --env API_KEY=test go run ./cmd/server/    # with env vars
```
