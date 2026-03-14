package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChecker_BasicStatus(t *testing.T) {
	c := NewChecker()
	s := c.Check()
	if s.Status != "ok" {
		t.Errorf("status = %q, want ok", s.Status)
	}
	if s.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestChecker_WithToolCount(t *testing.T) {
	c := NewChecker(WithToolCount(func() int { return 42 }))
	s := c.Check()
	if s.ToolCount != 42 {
		t.Errorf("tool_count = %d, want 42", s.ToolCount)
	}
}

func TestChecker_WithTaskCount(t *testing.T) {
	c := NewChecker(WithTaskCount(func() int { return 5 }))
	s := c.Check()
	if s.Tasks != 5 {
		t.Errorf("tasks = %d, want 5", s.Tasks)
	}
}

func TestChecker_WithCircuits(t *testing.T) {
	circuits := map[string]string{"api": "closed", "db": "open"}
	c := NewChecker(WithCircuits(func() map[string]string { return circuits }))
	s := c.Check()
	if len(s.Circuits) != 2 {
		t.Errorf("circuits count = %d, want 2", len(s.Circuits))
	}
	if s.Circuits["api"] != "closed" {
		t.Errorf("api circuit = %q, want closed", s.Circuits["api"])
	}
}

func TestHandler_Health(t *testing.T) {
	c := NewChecker(WithToolCount(func() int { return 10 }))
	h := Handler(c)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var s Status
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if s.Status != "ok" {
		t.Errorf("status = %q, want ok", s.Status)
	}
	if s.ToolCount != 10 {
		t.Errorf("tool_count = %d, want 10", s.ToolCount)
	}
}

func TestHandler_Ready(t *testing.T) {
	c := NewChecker()
	h := Handler(c)

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHandler_Live(t *testing.T) {
	c := NewChecker()
	h := Handler(c)

	req := httptest.NewRequest("GET", "/live", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
