# Changelog — v0.4.0 (Bridge + Gateway)

Release date: TBD

37 commits since v0.3.0. All 53 packages pass (3,358 tests). Zero `go vet` warnings.

## Added

### bridge/a2a — Bidirectional MCP-A2A Bridge

First production-quality Go library bridging MCP and A2A protocols. MCP tools are exposed as A2A skills; A2A agents can be consumed as MCP tools.

- **Translator**: Bidirectional type mapping — `ToolDefinition` to `AgentSkill`, `CallToolResult` to `Artifact`, MCP error codes to A2A `TaskStatus`, A2A `Message` to MCP call parameters
- **Executor**: Dispatches A2A task messages to the mcpkit `ToolRegistry`, returning A2A-formatted results
- **AgentCard**: Generates A2A-compliant agent cards from a registry's tool inventory
- **RemoteAgent**: Wraps a remote A2A agent as an MCP tool, enabling MCP clients to delegate to autonomous agents
- **Auth**: Cross-protocol OAuth/bearer token forwarding middleware
- **Streaming**: A2A SSE streaming support for long-running tool invocations

### bridge/openapi — OpenAPI-to-MCP Auto-Bridge

Auto-registers MCP tools from OpenAPI v3 specifications. Each API operation becomes an invocable tool.

- Parses OpenAPI v3 specs via `kin-openapi/openapi3`
- Configurable tool naming: `operationId` or `path_method` style
- Auth forwarding (header-based), configurable timeouts, custom HTTP client support
- Separate translator and handler layers for clean separation of concerns

### gateway/multi — Multi-Protocol Gateway

HTTP router with automatic protocol detection and canonical routing to MCP, A2A, and OpenAI adapters.

- **Router**: Concurrent-safe HTTP handler that detects incoming protocol and dispatches to the correct adapter
- **Protocol detection**: Inspects request body (up to 512 bytes) to classify MCP JSON-RPC, A2A, or OpenAI function-calling requests
- **MCP adapter**: Native MCP JSON-RPC handling backed by the `ToolRegistry`
- **A2A adapter**: Translates A2A task requests to tool invocations via the bridge translator
- **OpenAI adapter**: Maps OpenAI function-calling format to MCP tool calls

### gateway/adapter — Protocol Adapter Interface

Pluggable `ProtocolAdapter` interface and adapter registry for extending the gateway with new protocols.

- **ProtocolAdapter interface**: `Name()`, `Detect()`, `Handle()` contract for protocol plugins
- **A2A adapter**: Wraps `mcpkit/a2a` client as a `ProtocolAdapter`
- **OpenAPI adapter**: Wraps the OpenAPI bridge as a `ProtocolAdapter`

### testing/tck — Technology Compatibility Kit

Go-native conformance suite validating that mcpkit servers meet framework-level guarantees.

- 12 compliance checks across 2 categories:
  - **tools** (8): non-empty list, descriptions present, valid `InputSchema`, handler contract (`result, nil` — never `nil, error`), coded error format, non-empty names, no whitespace in names, non-nil handlers
  - **lifecycle** (4): initialize response, capabilities declared, modules registered, concurrent registry safety (race-detector compatible)
- `Suite.Run(t)` for full validation, `Suite.RunCategory(t, category)` for targeted checks
- Extensible via `Suite.AddCheck()`

### testing/conformance — MCP Everything-Server

Reference server implementing all testable MCP capabilities for the official MCP conformance suite.

- 18 tools: `echo`, `add`, `longRunningOperation`, `sampleLLM`, `getTinyImage`, `annotatedMessage`, `logMessage`, plus 11 official conformance tools (`test_simple_text`, `test_image_content`, `test_audio_content`, `test_embedded_resource`, `test_multiple_content_types`, `test_tool_with_logging`, `test_error_handling`, `test_tool_with_progress`, `test_sampling`, `test_elicitation`, `test_elicitation_sep1034_defaults`, `test_elicitation_sep1330_enums`)
- 2 static resources (text, binary PNG) + 2 resource templates (dynamic echo, parameterized JSON)
- 8 prompts: simple, complex with arguments, embedded resource, image, plus 4 official conformance prompts
- Completion providers for prompt arguments and resource template parameters
- Standalone server binary at `testing/conformance/cmd/`

### testing/benchmark — Cross-Protocol Benchmark Suite

Performance regression detection for bridge and gateway paths.

- 14 benchmarks (7 bridge, 7 gateway) measuring translation latency, throughput, and memory
- Performance targets: <500us/translation, <5ms card generation for 100 tools, <10ms gateway round-trip, >10K req/s throughput, <4KB allocation per translation
- `benchstat`-compatible output for CI regression detection

### middleware/truncate — Response Truncation

Limits tool response text content to a configurable byte budget.

- Configurable `MaxBytes` (default 4096) with hard ceiling (16384)
- Appends guidance message directing the model to use more specific queries
- Zero-overhead passthrough for responses under the limit

### middleware/debug — Debug Logging

Structured logging middleware with correlation IDs for tool call tracing.

- Logs tool name, input parameters (sensitive fields redacted), execution time, output size, error status
- Toggle via `MCPKIT_DEBUG=1` environment variable or `Config.Enabled`
- Zero-overhead passthrough when disabled
- Request correlation IDs propagated via context

### examples/a2a-bridge — Working Bridge Example

End-to-end example demonstrating the MCP-A2A bridge with a multi-tool server exposed as an A2A agent.

### examples/gateway — Gateway Example

Example demonstrating multi-protocol gateway setup with MCP, A2A, and OpenAI adapters.

### examples/truncate-demo — Truncation Middleware Demo

Example showing response truncation middleware in action.

## Changed

### device — Cross-Platform Device Abstraction

- Added macOS and Windows device providers (evdev rewrite, CoreMIDI, WinMM)
- Extracted shared MIDI parser with 25 tests, refactored darwin provider
- Added `DeviceFeedback` — MIDI output (Linux + macOS) + rumble (Linux + Windows)
- Stream Deck LED output (macOS) + WinMM MIDI output (Windows)
- Auto-reconnect on disconnect + hot-plug watchers for macOS/Windows
- `Grabbable` interface for macOS
- Expanded device classification database (Intech Studio Grid EN16)
- `EventKey`, `EventEncoder`, `EventMIDISysEx` + netlink hot-plug

### transport — Coverage Improvement

- Transport test coverage: 44.9% to 88.7% (unix socket server/client, `handleConn`)

### A2A Integration

- Integrated official A2A Go SDK v2.1.0 with compatibility layer
- A2A observability bridge
- A2A security — auth interceptor + rate limiting
- Absorbed `a2a-go` reference and `open-multi-agent` into mcpkit monorepo

### MCP Spec Compliance

- Closed remaining MCP spec compliance gaps

## Infrastructure

- Inlined reusable CI workflow from `.github` org repo
- Added benchmark CI workflow for gateway + OpenAPI adapter

## Stats

| Metric | Value |
|--------|-------|
| Packages tested | 53 |
| Total tests | 3,358 |
| Test failures | 0 |
| `go vet` warnings | 0 |
| Commits since v0.3.0 | 37 |
| New packages | 9 (`bridge/a2a`, `bridge/openapi`, `gateway/multi`, `gateway/adapter`, `testing/tck`, `testing/conformance`, `testing/benchmark`, `middleware/truncate`, `middleware/debug`) |
