// Package handoff implements the agent delegation protocol for MCP servers.
// It supports manager/agent-as-tool patterns where a manager agent can
// delegate tasks to sub-agents exposed as tools, as well as peer-to-peer
// delegation. DelegateMiddleware and WrapDelegate integrate with the registry
// middleware chain.
package handoff
