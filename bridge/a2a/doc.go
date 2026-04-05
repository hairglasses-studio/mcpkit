// Package a2a implements a bidirectional bridge between MCP (Model Context Protocol)
// tool registries and the A2A (Agent-to-Agent) Protocol v1.0.
//
// The bridge enables mcpkit-based MCP servers to participate in A2A agent networks.
// MCP tools are exposed as A2A skills, allowing remote A2A clients to discover and
// invoke deterministic tool handlers. Conversely, A2A agents can be consumed as MCP
// tools, enabling MCP clients to delegate work to autonomous agents.
//
// This package uses the official A2A Go SDK types from github.com/a2aproject/a2a-go/v2
// for protocol fidelity. Translation between MCP and A2A data models is handled by
// the [Translator] type, which maps:
//
//   - MCP [registry.ToolDefinition] to A2A AgentSkill (tool-to-skill)
//   - MCP CallToolResult to A2A Artifact (result-to-artifact)
//   - MCP error codes to A2A TaskStatus with appropriate failure states
//   - A2A Message to MCP tool call parameters (message-to-request)
//
// The package follows mcpkit conventions: error handling via handler.CodedErrorResult,
// thread safety via sync.RWMutex, and testing via stdlib testing.
//
// This is the first production-quality Go library bridging MCP and A2A protocols.
package a2a
