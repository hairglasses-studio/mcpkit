// Package a2h provides types for the Agent-to-Human (A2H) protocol.
//
// This is a placeholder package tracking the HumanLayer specification for
// structured agent-to-human communication. The A2H protocol defines how
// agents request human input, approval, and escalation in a standardized way.
//
// See: https://humanlayer.dev for the HumanLayer spec.
//
// Status: PLACEHOLDER - types defined, no implementation yet.
package a2h

import "time"

// RequestType classifies the kind of human interaction requested.
type RequestType string

const (
	// RequestTypeApproval asks the human to approve or reject an action.
	RequestTypeApproval RequestType = "approval"
	// RequestTypeInput asks the human to provide free-form input.
	RequestTypeInput RequestType = "input"
	// RequestTypeChoice asks the human to select from options.
	RequestTypeChoice RequestType = "choice"
	// RequestTypeEscalation notifies the human that the agent needs help.
	RequestTypeEscalation RequestType = "escalation"
)

// Status tracks the lifecycle of a human interaction request.
type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusCompleted Status = "completed"
	StatusTimedOut  Status = "timed_out"
	StatusCancelled Status = "cancelled"
)

// Request represents an agent's request for human interaction.
type Request struct {
	// ID uniquely identifies this request.
	ID string `json:"id"`
	// Type classifies the interaction.
	Type RequestType `json:"type"`
	// AgentID identifies the requesting agent.
	AgentID string `json:"agent_id"`
	// ThreadID links to the agent's execution thread.
	ThreadID string `json:"thread_id,omitempty"`
	// Message is the human-readable request.
	Message string `json:"message"`
	// Context provides additional information for the human.
	Context string `json:"context,omitempty"`
	// Choices are the available options for choice-type requests.
	Choices []string `json:"choices,omitempty"`
	// Urgency indicates the priority level.
	Urgency string `json:"urgency,omitempty"`
	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the request times out. Zero means no expiration.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// Metadata contains optional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Response represents a human's response to an agent request.
type Response struct {
	// RequestID links to the original request.
	RequestID string `json:"request_id"`
	// Status is the outcome of the human interaction.
	Status Status `json:"status"`
	// Value is the human's response (free-text, selected choice, etc.).
	Value string `json:"value,omitempty"`
	// Reason is an optional explanation for the decision.
	Reason string `json:"reason,omitempty"`
	// RespondedAt is when the response was provided.
	RespondedAt time.Time `json:"responded_at"`
}

// Channel is the interface for a human interaction backend.
// Implementations might use Slack, email, desktop notifications, web UI, etc.
//
// Status: PLACEHOLDER - interface defined, no implementations yet.
type Channel interface {
	// Send delivers a request to the human and returns immediately.
	// The response will be delivered asynchronously via the ResponseHandler.
	Send(req Request) error
	// Name returns the channel identifier (e.g., "slack", "desktop", "web").
	Name() string
}

// ResponseHandler is called when a human responds to a request.
// Status: PLACEHOLDER - callback type defined, not yet wired.
type ResponseHandler func(resp Response)
