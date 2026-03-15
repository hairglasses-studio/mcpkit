package dispatcher

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Priority determines job execution order. Higher values execute first.
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// Job represents a tool call queued for execution.
type Job struct {
	Name     string
	TD       registry.ToolDefinition
	Ctx      context.Context
	Request  registry.CallToolRequest
	Handler  registry.ToolHandlerFunc
	Priority Priority
	Group    string
	seq      uint64 // monotonic sequence number for FIFO within same priority
	result   chan jobResult
}

// jobResult carries the handler's return values through the result channel.
type jobResult struct {
	Result *registry.CallToolResult
	Err    error
}
