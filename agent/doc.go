// Package agent provides core types for agent execution threads and events.
//
// A Thread is an append-only event log that records the complete history of
// an agent's execution. Events represent discrete state changes (tool calls,
// LLM responses, errors, human input, etc.). The Reduce function applies an
// event to a thread, producing a new thread state.
//
// This package follows the 12-Factor Agents pattern of maintaining an
// immutable, append-only execution log that can be replayed for debugging,
// auditing, and recovery.
package agent
