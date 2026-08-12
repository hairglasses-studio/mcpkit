# OpenAPI-to-MCP Integration Guide

The `mcpkit/bridge/openapi` package automatically bridges OpenAPI v3 specifications to MCP (Model Context Protocol) tool registries. It parses OpenAPI endpoints, converts API operations into typed MCP tool definitions, and proxies tool invocations to the upstream REST API without requiring custom handler code for every endpoint.

---

## Architecture Overview

```
┌────────────────────────┐      ┌─────────────────────────┐      ┌────────────────────────┐
│ OpenAPI v3 Spec        │ ───► │ openapi.Bridge          │ ───► │ mcpkit ToolRegistry    │
│ (URL, file, or struct) │      │ (loader + translator)   │      │ (registered module)    │
└────────────────────────┘      └─────────────────────────┘      └────────────────────────┘
                                             │                                │
                                             ▼                                ▼
                                ┌─────────────────────────┐      ┌────────────────────────┐
                                │ Upstream REST API       │ ◄─── │ MCPServer              │
                                │ (HTTP client proxy)     │      │ (stdio or HTTP)        │
                                └─────────────────────────┘      └────────────────────────┘
```

1. **Spec Loading**: Parses an OpenAPI v3 YAML or JSON spec from a URL, local file, or in-memory `*openapi3.T` struct via `github.com/getkin/kin-openapi`.
2. **Operation Discovery**: Iterates over all paths and HTTP methods (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`).
3. **Tool Generation**: Maps each operation to a `registry.ToolDefinition` with JSON Schema parameters derived from path, query, header parameters and request body.
4. **Proxy Handler**: Generates a `registry.ToolHandlerFunc` that extracts arguments, substitutes path parameters, appends query parameters, forwards auth headers, posts body payloads, and returns formatted responses as MCP `CallToolResult`.

---

## Configuration Reference

`openapi.BridgeConfig` controls how OpenAPI operations are converted and executed:

```go
type BridgeConfig struct {
    // BaseURL overrides the spec's server URL for all requests.
    BaseURL string

    // NameStyle controls tool name generation.
    // "operationId" (default): use operation.OperationID.
    // "path_method": generate from method + path (e.g. "get_pets_id").
    NameStyle string

    // Timeout for upstream HTTP requests. Default: 30s.
    Timeout time.Duration

    // AuthHeader is the header name for authentication (e.g. "Authorization" or "X-API-Key").
    AuthHeader string

    // AuthToken is the credential value sent in AuthHeader (e.g. "Bearer sk-...").
    AuthToken string

    // Client is an optional custom HTTP client. If nil, a default client with Timeout is created.
    Client *http.Client
}
```

---

## Operation-to-Tool Mapping Rules

### Tool Naming
- **`operationId` style** (default): Uses `operation.OperationID` directly if non-empty (e.g., `listPets`, `createPet`).
- **`path_method` style**: Constructs names like `get_pets` or `get_pets_id` by lowercasing the method and sanitizing path slashes and braces.

### Parameter Mapping

| OpenAPI Parameter Type | Tool Input Parameter | Proxy Behavior |
|---|---|---|
| **Path Parameter** (`in: path`) | Registered as string parameter | Substituted into URL path placeholders (e.g., `/pets/{petId}` -> `/pets/42`) |
| **Query Parameter** (`in: query`) | Registered as string parameter | Appended as URL query parameters (e.g., `?limit=10&status=available`) |
| **Header Parameter** (`in: header`) | Registered as string parameter | Injected directly into HTTP request headers |
| **Request Body** (`requestBody`) | Registered as `body` string or object parameter | JSON-encoded and sent as `Content-Type: application/json` body |

### Response Formatting
- **2xx Success**: Response body is capped at 1 MB, pretty-printed if valid JSON, and returned as an MCP text content block.
- **4xx/5xx Error**: HTTP errors return as an MCP coded error (`handler.ErrAPIError`) with status code and truncated error response snippet.


---

## Usage Examples

### 1. Basic Spec Loading from File

```go
package main

import (
    "log"

    "github.com/hairglasses-studio/mcpkit/bridge/openapi"
    "github.com/hairglasses-studio/mcpkit/registry"
)

func main() {
    reg := registry.NewToolRegistry()

    bridge, err := openapi.NewBridge("./openapi.json", reg, openapi.BridgeConfig{
        BaseURL:    "https://api.example.com/v1",
        NameStyle:  "operationId",
        AuthHeader: "Authorization",
        AuthToken:  "Bearer sk-my-secret-token",
    })
    if err != nil {
        log.Fatalf("Failed to create OpenAPI bridge: %v", err)
    }

    if err := bridge.RegisterTools(); err != nil {
        log.Fatalf("Failed to register tools: %v", err)
    }

    s := registry.NewMCPServer("openapi-server", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

### 2. Loading Pre-Parsed Spec in Code

```go
spec, err := openapi3.NewLoader().LoadFromData(specJSON)
if err != nil {
    log.Fatal(err)
}

bridge, err := openapi.NewBridgeFromSpec(spec, reg, openapi.BridgeConfig{
    BaseURL: "http://localhost:8080",
})
if err != nil {
    log.Fatal(err)
}

if err := bridge.RegisterTools(); err != nil {
    log.Fatal(err)
}
```

---

## Best Practices & Safety

1. **Authentication Security**: Never hardcode API tokens in source code or specs. Inject `AuthToken` from environment variables (`os.Getenv(...)`).
2. **Timeouts & Deadlines**: Always configure an explicit `Timeout` on `BridgeConfig` (default is 30s) to prevent hanging tool calls.
3. **Response Payload Truncation**: For APIs that return large JSON datasets, combine `bridge/openapi` with `mcpkit/middleware.Truncate` to keep tool outputs within context budgets.
4. **Tool Naming Conventions**: Ensure `operationId` values in your OpenAPI spec follow clean naming conventions (`snake_case` or `camelCase`) so LLMs can easily select and invoke them.
5. **Gateway Aggregation**: Use `mcpkit/gateway` to combine OpenAPI-derived tools with native Go tools or remote MCP servers into a single namespaced registry.

---

## Complete Executable Example

A complete, runnable server example with a mock HTTP backend and OpenAPI bridge is available in [`examples/openapi/main.go`](../examples/openapi/main.go):

```bash
go run ./examples/openapi
```
