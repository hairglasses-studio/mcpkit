package health

import (
	"encoding/json"
	"net/http"
	"sync"
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
	mu        sync.RWMutex
	startedAt time.Time
	status    string
	toolCount func() int
	taskCount func() int
	circuits  func() map[string]string
}

// SetStatus sets the overall lifecycle status (e.g., "healthy", "draining", "stopped").
func (c *Checker) SetStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// Status returns the current lifecycle status. Returns "healthy" if not explicitly set.
func (c *Checker) Status() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.status == "" {
		return "healthy"
	}
	return c.status
}

// IsReady reports whether the server is ready to serve traffic.
// Returns false when status is "draining" or "stopped".
func (c *Checker) IsReady() bool {
	s := c.Status()
	return s != "draining" && s != "stopped"
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
		if !c.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})

	return mux
}
