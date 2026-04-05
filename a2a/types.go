package a2a

import (
	"encoding/json"
	"time"
)

// TaskState represents the lifecycle state of an A2A task.
type TaskState string

const (
	TaskSubmitted  TaskState = "submitted"
	TaskWorking    TaskState = "working"
	TaskInputNeeded TaskState = "input-needed"
	TaskCompleted  TaskState = "completed"
	TaskCanceled   TaskState = "canceled"
	TaskFailed     TaskState = "failed"
)

// IsTerminal returns true if the task state is a terminal state.
func (s TaskState) IsTerminal() bool {
	return s == TaskCompleted || s == TaskCanceled || s == TaskFailed
}

// Task represents an A2A task — the primary unit of work.
type Task struct {
	ID       string            `json:"id"`
	State    TaskState         `json:"state"`
	Messages []Message         `json:"messages,omitempty"`
	Artifacts []Artifact       `json:"artifacts,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
}

// Message represents a conversation turn in an A2A task.
type Message struct {
	Role  string `json:"role"` // "user" or "agent"
	Parts []Part `json:"parts"`
}

// Part is a piece of content within a message.
type Part struct {
	Type     string          `json:"type"` // "text", "file", "data"
	Text     string          `json:"text,omitempty"`
	File     *FileContent    `json:"file,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TextPart creates a text Part.
func TextPart(text string) Part {
	return Part{Type: "text", Text: text}
}

// FileContent represents a file attachment.
type FileContent struct {
	Name     string `json:"name"`
	MIMEType string `json:"mimeType,omitempty"`
	Bytes    []byte `json:"bytes,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Artifact represents an output produced by a task.
type Artifact struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parts       []Part `json:"parts"`
	Index       int    `json:"index"`
	Append      bool   `json:"append,omitempty"`
	LastChunk   bool   `json:"lastChunk,omitempty"`
}

// AgentCard describes an A2A agent's capabilities for discovery.
type AgentCard struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	URL          string   `json:"url"`
	Version      string   `json:"version,omitempty"`
	Provider     *Provider `json:"provider,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
	Skills       []Skill  `json:"skills,omitempty"`
	Auth         *AuthConfig `json:"authentication,omitempty"`
}

// Provider identifies the organization running the agent.
type Provider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// Capabilities describes what the agent supports.
type Capabilities struct {
	Streaming        bool `json:"streaming,omitempty"`
	PushNotifications bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

// Skill describes one capability of the agent (maps to MCP tool).
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// AuthConfig describes authentication requirements.
type AuthConfig struct {
	Schemes []string `json:"schemes"` // "bearer", "oauth2", "apiKey"
}

// TaskSendParams are the parameters for sending a task.
type TaskSendParams struct {
	ID       string    `json:"id"`
	Messages []Message `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TaskQueryParams are the parameters for querying a task.
type TaskQueryParams struct {
	ID string `json:"id"`
}

// JSONRPCRequest is a JSON-RPC 2.0 request (A2A uses JSON-RPC).
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
