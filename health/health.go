// Package health provides an HTTP health check endpoint and checker registry
// for exposing server liveness and readiness status.
package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Status represents the health status.
type Status struct {
	Status    string            `json:"status"`
	Uptime    string            `json:"uptime"`
	ToolCount int               `json:"tool_count"`
	Tasks     int               `json:"active_tasks,omitempty"`
	Circuits  map[string]string `json:"circuit_breakers,omitempty"`
}

// Checker provides health check data.
type Checker struct {
	startedAt time.Time
	toolCount func() int
	taskCount func() int
	circuits  func() map[string]string
}

// Option configures a Checker.
type Option func(*Checker)

// WithToolCount provides a function to get the current tool count.
func WithToolCount(fn func() int) Option {
	return func(c *Checker) { c.toolCount = fn }
}

// WithTaskCount provides a function to get the current active task count.
func WithTaskCount(fn func() int) Option {
	return func(c *Checker) { c.taskCount = fn }
}

// WithCircuits provides a function to get circuit breaker states.
func WithCircuits(fn func() map[string]string) Option {
	return func(c *Checker) { c.circuits = fn }
}

// NewChecker creates a new health checker.
func NewChecker(opts ...Option) *Checker {
	c := &Checker{
		startedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Check returns the current health status.
func (c *Checker) Check() Status {
	s := Status{
		Status: "ok",
		Uptime: time.Since(c.startedAt).Truncate(time.Second).String(),
	}
	if c.toolCount != nil {
		s.ToolCount = c.toolCount()
	}
	if c.taskCount != nil {
		s.Tasks = c.taskCount()
	}
	if c.circuits != nil {
		s.Circuits = c.circuits()
	}
	return s
}

// Handler returns an http.Handler that serves health endpoints.
// It responds to /health, /ready, and /live.
func Handler(c *Checker) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c.Check())
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})

	return mux
}
