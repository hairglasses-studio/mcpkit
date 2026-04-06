package a2h

import (
	"testing"
	"time"
)

func TestRequestType_Constants(t *testing.T) {
	types := []RequestType{
		RequestTypeApproval,
		RequestTypeInput,
		RequestTypeChoice,
		RequestTypeEscalation,
	}

	seen := make(map[RequestType]bool)
	for _, rt := range types {
		if rt == "" {
			t.Error("request type constant should not be empty")
		}
		if seen[rt] {
			t.Errorf("duplicate request type: %q", rt)
		}
		seen[rt] = true
	}
}

func TestStatus_Constants(t *testing.T) {
	statuses := []Status{
		StatusPending,
		StatusApproved,
		StatusRejected,
		StatusCompleted,
		StatusTimedOut,
		StatusCancelled,
	}

	seen := make(map[Status]bool)
	for _, s := range statuses {
		if s == "" {
			t.Error("status constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate status: %q", s)
		}
		seen[s] = true
	}
}

func TestRequest_Fields(t *testing.T) {
	now := time.Now()
	req := Request{
		ID:        "req-001",
		Type:      RequestTypeApproval,
		AgentID:   "agent-1",
		ThreadID:  "thread-1",
		Message:   "Deploy to production?",
		Context:   "All tests pass. Coverage 95%.",
		Choices:   nil,
		Urgency:   "high",
		CreatedAt: now,
		Metadata:  map[string]string{"env": "prod"},
	}

	if req.ID != "req-001" {
		t.Errorf("ID = %q", req.ID)
	}
	if req.Type != RequestTypeApproval {
		t.Errorf("Type = %q", req.Type)
	}
	if req.AgentID != "agent-1" {
		t.Errorf("AgentID = %q", req.AgentID)
	}
}

func TestResponse_Fields(t *testing.T) {
	now := time.Now()
	resp := Response{
		RequestID:   "req-001",
		Status:      StatusApproved,
		Value:       "yes",
		Reason:      "Looks good",
		RespondedAt: now,
	}

	if resp.RequestID != "req-001" {
		t.Errorf("RequestID = %q", resp.RequestID)
	}
	if resp.Status != StatusApproved {
		t.Errorf("Status = %q", resp.Status)
	}
	if resp.Value != "yes" {
		t.Errorf("Value = %q", resp.Value)
	}
}
