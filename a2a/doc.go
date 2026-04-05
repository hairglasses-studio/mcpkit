// Package a2a implements an A2A (Agent-to-Agent) Protocol v1.0 bridge for mcpkit.
//
// The A2A protocol (https://github.com/a2aproject/A2A) enables vendor-agnostic
// communication between AI agents. This package bridges MCP tool calls with A2A
// task delegation, allowing mcpkit-based servers to both send tasks to and
// receive tasks from A2A agents.
//
// Key components:
//
//   - [AgentCard] generates an A2A agent card from an MCP tool registry
//   - [Client] sends tasks to remote A2A agents
//   - [Server] accepts A2A tasks and dispatches them as MCP tool calls
//   - [Bridge] provides bidirectional MCP↔A2A translation
//
// This is the first Go implementation of an MCP↔A2A bridge.
package a2a
