//go:build !official_sdk

package hitools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Decision is the human's verdict on an approval request.
type Decision string

const (
	// Approved means the human granted approval to proceed.
	Approved Decision = "approved"
	// Denied means the human rejected the request.
	Denied Decision = "denied"
	// Modified means the human approved with modifications (see Value/Comment).
	Modified Decision = "modified"
)

// ApprovalRequest represents a structured request for human approval before
// a tool call executes. This implements 12-Factor Agent Factor 7: "Contact
// Humans with Tool Calls" — human contact is a tool, not special-case code.
type ApprovalRequest struct {
	// ID uniquely identifies this approval request.
	ID string `json:"id"`
	// ToolName is the tool that needs approval to execute.
	ToolName string `json:"tool_name"`
	// Action describes what the tool wants to do.
	Action string `json:"action"`
	// Context explains why the tool needs to do it.
	Context string `json:"context"`
	// Urgency controls notification routing (low, normal, high, critical).
	Urgency ApprovalUrgency `json:"urgency"`
	// Options provides structured choices for the human.
	Options []ApprovalOption `json:"options,omitempty"`
	// ExpiresAt is the deadline for a response. Zero means no timeout.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`
}

// ApprovalUrgency controls notification channel routing.
type ApprovalUrgency string

const (
	ApprovalUrgencyLow      ApprovalUrgency = "low"
	ApprovalUrgencyNormal   ApprovalUrgency = "normal"
	ApprovalUrgencyHigh     ApprovalUrgency = "high"
	ApprovalUrgencyCritical ApprovalUrgency = "critical"
)

// ApprovalOption is one of the choices offered to the human.
type ApprovalOption struct {
	// Label is the display text for this option.
	Label string `json:"label"`
	// Value is the machine-readable value returned if selected.
	Value string `json:"value"`
	// Description provides additional context about this option.
	Description string `json:"description,omitempty"`
	// IsDefault marks this option as the pre-selected default.
	IsDefault bool `json:"is_default,omitempty"`
}

// ApprovalResponse is the human's decision on an approval request.
type ApprovalResponse struct {
	// RequestID links this response to the original ApprovalRequest.
	RequestID string `json:"request_id"`
	// Decision is the human's verdict (approved, denied, modified).
	Decision Decision `json:"decision"`
	// Value is the selected option value (for requests with options).
	Value string `json:"value,omitempty"`
	// Comment is an optional human-written note explaining the decision.
	Comment string `json:"comment,omitempty"`
	// Timestamp is when the response was recorded.
	Timestamp time.Time `json:"timestamp"`
}

// ApprovalStore manages the lifecycle of approval requests and responses.
// Implementations must be safe for concurrent use.
type ApprovalStore interface {
	// Submit creates a new approval request. Returns an error if the request
	// ID already exists.
	Submit(ctx context.Context, req ApprovalRequest) error
	// Respond records a human's decision for a pending approval request.
	// Returns an error if the request ID does not exist or is already resolved.
	Respond(ctx context.Context, resp ApprovalResponse) error
	// Get retrieves an approval request by ID. Returns (nil, nil) if not found.
	Get(ctx context.Context, requestID string) (*ApprovalRequest, error)
	// GetResponse retrieves the response for a request. Returns (nil, nil) if
	// no response has been recorded yet.
	GetResponse(ctx context.Context, requestID string) (*ApprovalResponse, error)
	// Pending returns all unresolved approval requests.
	Pending(ctx context.Context) ([]ApprovalRequest, error)
}

// InMemoryApprovalStore is a thread-safe in-memory ApprovalStore implementation.
// Suitable for single-process deployments and testing.
type InMemoryApprovalStore struct {
	mu        sync.RWMutex
	requests  map[string]ApprovalRequest
	responses map[string]ApprovalResponse
}

// NewInMemoryApprovalStore creates a new empty in-memory approval store.
func NewInMemoryApprovalStore() *InMemoryApprovalStore {
	return &InMemoryApprovalStore{
		requests:  make(map[string]ApprovalRequest),
		responses: make(map[string]ApprovalResponse),
	}
}

// Submit stores a new approval request.
func (s *InMemoryApprovalStore) Submit(_ context.Context, req ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.requests[req.ID]; exists {
		return fmt.Errorf("hitools: approval request %q already exists", req.ID)
	}
	s.requests[req.ID] = req
	return nil
}

// Respond records a human's decision for a pending approval request.
func (s *InMemoryApprovalStore) Respond(_ context.Context, resp ApprovalResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.requests[resp.RequestID]; !exists {
		return fmt.Errorf("hitools: approval request %q not found", resp.RequestID)
	}
	if _, exists := s.responses[resp.RequestID]; exists {
		return fmt.Errorf("hitools: approval request %q already resolved", resp.RequestID)
	}
	s.responses[resp.RequestID] = resp
	return nil
}

// Get retrieves an approval request by ID.
func (s *InMemoryApprovalStore) Get(_ context.Context, requestID string) (*ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[requestID]
	if !ok {
		return nil, nil
	}
	return &req, nil
}

// GetResponse retrieves the response for a request.
func (s *InMemoryApprovalStore) GetResponse(_ context.Context, requestID string) (*ApprovalResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	resp, ok := s.responses[requestID]
	if !ok {
		return nil, nil
	}
	return &resp, nil
}

// Pending returns all approval requests that have not yet received a response.
func (s *InMemoryApprovalStore) Pending(_ context.Context) ([]ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pending []ApprovalRequest
	for id, req := range s.requests {
		if _, resolved := s.responses[id]; !resolved {
			pending = append(pending, req)
		}
	}
	return pending, nil
}

// generateApprovalID creates a random hex ID for approval requests.
func generateApprovalID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("hitools: generate approval ID: %w", err)
	}
	return "apr-" + hex.EncodeToString(b), nil
}

// ApprovalMiddlewareConfig configures the approval middleware.
type ApprovalMiddlewareConfig struct {
	// Store persists approval requests and responses.
	Store ApprovalStore
	// ShouldApprove returns true if the given tool requires human approval.
	// If nil, no tools require approval.
	ShouldApprove func(toolName string, td registry.ToolDefinition) bool
	// DefaultUrgency is the urgency level for approval requests when not
	// otherwise specified. Defaults to ApprovalUrgencyNormal.
	DefaultUrgency ApprovalUrgency
	// Timeout is the maximum time to wait for a human response. Zero means
	// block indefinitely (or until context cancellation).
	Timeout time.Duration
	// OnRequest is an optional callback invoked when an approval request is
	// created. Use this to emit thread events, send notifications, etc.
	OnRequest func(ctx context.Context, req ApprovalRequest)
	// OnResponse is an optional callback invoked when an approval response is
	// received. Use this to emit thread events, log decisions, etc.
	OnResponse func(ctx context.Context, req ApprovalRequest, resp ApprovalResponse)
}

// ApprovalMiddleware creates a registry.Middleware that intercepts tool calls
// requiring human approval. When ShouldApprove returns true for a tool, the
// middleware:
//  1. Creates an ApprovalRequest and stores it
//  2. Calls OnRequest (if set) to notify external systems
//  3. Polls the store for a response (with timeout)
//  4. Proceeds, denies, or returns an error based on the decision
//
// This implements 12-Factor Agent Factor 7: human contact is a tool call,
// not special-case code. The middleware uses structured requests with
// questions, options, and urgency.
func ApprovalMiddleware(config ApprovalMiddlewareConfig) registry.Middleware {
	if config.DefaultUrgency == "" {
		config.DefaultUrgency = ApprovalUrgencyNormal
	}

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Check if this tool requires approval.
			if config.ShouldApprove == nil || !config.ShouldApprove(name, td) {
				return next(ctx, req)
			}

			// Build the approval request.
			id, err := generateApprovalID()
			if err != nil {
				return registry.MakeErrorResult(
					fmt.Sprintf("[APPROVAL_ERROR] failed to generate request ID: %v", err),
				), nil
			}

			now := time.Now()
			approvalReq := ApprovalRequest{
				ID:        id,
				ToolName:  name,
				Action:    fmt.Sprintf("Execute tool %q", name),
				Context:   formatToolContext(td, req),
				Urgency:   config.DefaultUrgency,
				CreatedAt: now,
			}

			if config.Timeout > 0 {
				approvalReq.ExpiresAt = now.Add(config.Timeout)
			}

			// Submit to the store.
			if err := config.Store.Submit(ctx, approvalReq); err != nil {
				return registry.MakeErrorResult(
					fmt.Sprintf("[APPROVAL_ERROR] failed to submit request: %v", err),
				), nil
			}

			// Notify external systems.
			if config.OnRequest != nil {
				config.OnRequest(ctx, approvalReq)
			}

			// Wait for the response.
			resp, err := waitForApproval(ctx, config.Store, id, config.Timeout)
			if err != nil {
				return registry.MakeErrorResult(
					fmt.Sprintf("[APPROVAL_TIMEOUT] %v", err),
				), nil
			}

			// Notify about the response.
			if config.OnResponse != nil {
				config.OnResponse(ctx, approvalReq, *resp)
			}

			// Act on the decision.
			switch resp.Decision {
			case Approved, Modified:
				return next(ctx, req)
			case Denied:
				msg := "[APPROVAL_DENIED] tool call denied by human"
				if resp.Comment != "" {
					msg = fmt.Sprintf("[APPROVAL_DENIED] %s", resp.Comment)
				}
				return registry.MakeErrorResult(msg), nil
			default:
				return registry.MakeErrorResult(
					fmt.Sprintf("[APPROVAL_ERROR] unknown decision: %q", resp.Decision),
				), nil
			}
		}
	}
}

// waitForApproval polls the store for a response to the given request ID.
// It respects both the configured timeout and context cancellation.
func waitForApproval(ctx context.Context, store ApprovalStore, requestID string, timeout time.Duration) (*ApprovalResponse, error) {
	// Build a context with timeout if configured.
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Poll interval starts at 50ms and backs off to 500ms.
	interval := 50 * time.Millisecond
	maxInterval := 500 * time.Millisecond

	for {
		resp, err := store.GetResponse(ctx, requestID)
		if err != nil {
			return nil, fmt.Errorf("failed to check approval status: %w", err)
		}
		if resp != nil {
			return resp, nil
		}

		// Check if expired.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("approval request %q timed out or was cancelled", requestID)
		case <-time.After(interval):
		}

		// Back off.
		if interval < maxInterval {
			interval = interval * 2
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}

// formatToolContext builds a human-readable context string from the tool
// definition and request.
func formatToolContext(td registry.ToolDefinition, req registry.CallToolRequest) string {
	desc := td.Tool.Description
	if desc == "" {
		desc = td.Tool.Name
	}

	args := registry.ExtractArguments(req)
	if len(args) == 0 {
		return fmt.Sprintf("Tool: %s\nDescription: %s", td.Tool.Name, desc)
	}

	return fmt.Sprintf("Tool: %s\nDescription: %s\nArguments: %v", td.Tool.Name, desc, args)
}

// ThreadEventData holds the data payload for human request/response thread events.
// Use this as the Data field when appending EventHumanRequest or EventHumanResponse
// events to a session thread.
type ThreadEventData struct {
	// ApprovalID links this event to the ApprovalRequest/Response.
	ApprovalID string `json:"approval_id"`
	// ToolName is the tool that triggered the approval flow.
	ToolName string `json:"tool_name"`
	// Action describes what the tool wanted to do.
	Action string `json:"action,omitempty"`
	// Urgency is the request urgency level.
	Urgency ApprovalUrgency `json:"urgency,omitempty"`
	// Decision is the human's verdict (only set in response events).
	Decision Decision `json:"decision,omitempty"`
	// Comment is the human's optional note (only set in response events).
	Comment string `json:"comment,omitempty"`
}
