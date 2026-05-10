# FastMCP to mcpkit Migration Guide

FastMCP is a Python-first MCP framework built around decorators and automatic schema generation. mcpkit provides the same authoring shape in Go through `handler.TypedHandler`, `registry.ToolModule`, and package-level registries. Use this guide when a FastMCP server needs static binaries, Go middleware, stronger compile-time contracts, or shared production infrastructure.

## Concept Map

| FastMCP Python | mcpkit Go |
|---|---|
| `mcp = FastMCP("name")` | `reg := registry.NewToolRegistry()` plus `registry.NewMCPServer("name", version)` |
| `@mcp.tool` | `handler.TypedHandler[In, Out]` returned from `Tools()` |
| function signature annotations | Go input/output structs with `json` and `jsonschema` tags |
| function docstring | tool description string |
| `@mcp.resource("uri://...")` | `resources.ResourceDefinition` |
| `@mcp.prompt` | `prompts.PromptDefinition` |
| `mcp.run()` | `registry.ServeStdio(s)` |
| `mcp.run(transport="http")` | `transport`, `examples/http`, or a custom HTTP server |
| decorator metadata | `registry.ToolDefinition` fields: category, tags, version, runtime group, call type |

## Tool Migration

FastMCP:

```python
from fastmcp import FastMCP

mcp = FastMCP("calculator")

@mcp.tool
def add(a: int, b: int) -> int:
    """Add two numbers."""
    return a + b

if __name__ == "__main__":
    mcp.run()
```

mcpkit:

```go
package calculator

import (
    "context"

    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type AddInput struct {
    A int `json:"a" jsonschema:"required,description=First number"`
    B int `json:"b" jsonschema:"required,description=Second number"`
}

type AddOutput struct {
    Result int `json:"result"`
}

type Module struct{}

func (m *Module) Name() string        { return "calculator" }
func (m *Module) Description() string { return "Calculator tools" }

func (m *Module) Tools() []registry.ToolDefinition {
    return []registry.ToolDefinition{
        handler.TypedHandler[AddInput, AddOutput](
            "calculator_add",
            "Add two numbers.",
            func(ctx context.Context, in AddInput) (AddOutput, error) {
                return AddOutput{Result: in.A + in.B}, nil
            },
        ),
    }
}
```

Server wiring:

```go
package main

import (
    "log"

    "github.com/hairglasses-studio/mcpkit/registry"
    "github.com/my-org/my-server/calculator"
)

func main() {
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&calculator.Module{})

    s := registry.NewMCPServer("calculator", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

## Resources

FastMCP:

```python
@mcp.resource("config://app")
def get_config() -> dict:
    return {"theme": "dark", "version": "1.0"}
```

mcpkit:

```go
func (m *Module) Resources() []resources.ResourceDefinition {
    return []resources.ResourceDefinition{
        {
            Resource: mcp.NewResource("config://app", "App Config", mcp.WithMIMEType("application/json")),
            Handler: func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
                return []mcp.ResourceContents{
                    mcp.TextResourceContents{
                        URI:      req.Params.URI,
                        MIMEType: "application/json",
                        Text:     `{"theme":"dark","version":"1.0"}`,
                    },
                }, nil
            },
            Category: "config",
            Tags:     []string{"readonly"},
        },
    }
}
```

Resource support still has SDK-specific handler shapes. Keep resource modules behind the default SDK path until their official-SDK fixtures are migrated.

## Prompts

FastMCP:

```python
@mcp.prompt
def analyze_data(data_points: list[float]) -> str:
    return f"Analyze these data points: {data_points}"
```

mcpkit:

```go
func (m *Module) Prompts() []prompts.PromptDefinition {
    return []prompts.PromptDefinition{
        {
            Prompt: mcp.NewPrompt(
                "analyze_data",
                mcp.WithPromptDescription("Analyze numeric data points"),
                mcp.WithArgument("data_points", mcp.RequiredArgument()),
            ),
            Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
                data := req.Params.Arguments["data_points"]
                return mcp.NewGetPromptResult("Analyze data", []mcp.PromptMessage{
                    mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Analyze these data points: "+data)),
                }), nil
            },
        },
    }
}
```

## Migration Checklist

1. Group FastMCP decorators by domain and create one Go `ToolModule` per domain.
2. Convert each Python function signature into an input struct and output struct.
3. Use `handler.TypedHandler` for every tool unless the handler needs raw MCP request access.
4. Move FastMCP decorator metadata into `ToolDefinition` fields: `Category`, `Tags`, `Version`, `IsWrite`, and `CallType`.
5. Replace decorator-based startup with a small `cmd/<server>/main.go` that registers modules and calls `registry.ServeStdio`.
6. Add `mcptest` integration tests for each migrated tool.
7. Run:

   ```bash
   go test ./... -count=1
   make build-official
   make test-official
   ```

## When To Keep FastMCP

Keep FastMCP when the server is mostly Python data-science code, relies on dynamic runtime imports, or needs the smallest possible script for personal use. Move to mcpkit when the server needs static deployment, Go middleware, cross-package registries, stronger CI gates, or long-lived production ownership.
