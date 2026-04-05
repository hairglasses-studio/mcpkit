// Package handoff provides agent delegation — register named sub-agents and
// dispatch work to them with depth limiting, timeout enforcement, and optional
// cost tracking via finops.
package handoff

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

var (
	// ErrAgentNotFound is returned when the named agent is not registered.
	ErrAgentNotFound = errors.New("handoff: agent not found")
	// ErrDelegationDepth is returned when the max delegation chain depth is exceeded.
	ErrDelegationDepth = errors.New("handoff: max delegation depth exceeded")
	// ErrDelegateTimeout is returned when the delegation times out.
	ErrDelegateTimeout = errors.New("handoff: delegation timed out")
	// ErrNoDelegateFunc is returned when Delegate is called but no DelegateFunc was configured.
	ErrNoDelegateFunc = errors.New("handoff: no delegate function configured")
)

// AgentRef describes a delegatable agent.
type AgentRef struct {
	Name        string
	Description string
	Skills      []string
	Registry    *registry.ToolRegistry
	Sampler     sampling.SamplingClient
	Metadata    map[string]string
}

// HandoffRequest defines work to be delegated to an agent.
type HandoffRequest struct {
	TaskDescription string
	Context         map[string]string
	MaxIterations   int
	Timeout         time.Duration
}

// HandoffResult captures the outcome of a delegation.
type HandoffResult struct {
	AgentName  string        `json:"agent_name"`
	Status     string        `json:"status"` // completed, failed, timeout
	Result     string        `json:"result"`
	Iterations int           `json:"iterations"`
	Duration   time.Duration `json:"duration_ns"`
	TokensUsed int64         `json:"tokens_used,omitempty"`
}

// DelegateFunc executes work against an agent. Decoupled from any specific
// loop runner so callers can inject ralph, a mock, or any future runner.
type DelegateFunc func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error)

// Config configures the HandoffManager.
type Config struct {
	// DefaultTimeout is applied when HandoffRequest.Timeout is zero.
	DefaultTimeout time.Duration
	// MaxDelegationDepth prevents infinite delegation chains (default 5).
	MaxDelegationDepth int
	// CostTracker records token usage for each delegation if non-nil.
	CostTracker *finops.Tracker
	// Delegate is the function used to execute work on an agent. Required for Delegate calls.
	Delegate DelegateFunc
}

// HandoffManager manages agent references and delegation.
type HandoffManager struct {
	mu     sync.RWMutex
	agents map[string]AgentRef
	config Config
}

// NewHandoffManager creates a HandoffManager with the given config.
// Sensible defaults are applied for missing fields.
func NewHandoffManager(config ...Config) *HandoffManager {
	cfg := Config{
		DefaultTimeout:     30 * time.Second,
		MaxDelegationDepth: 5,
	}
	if len(config) > 0 {
		cfg = config[0]
		if cfg.MaxDelegationDepth <= 0 {
			cfg.MaxDelegationDepth = 5
		}
		if cfg.DefaultTimeout <= 0 {
			cfg.DefaultTimeout = 30 * time.Second
		}
	}
	return &HandoffManager{
		agents: make(map[string]AgentRef),
		config: cfg,
	}
}

// Register adds an agent. Returns an error if the name is empty or already registered.
func (m *HandoffManager) Register(agent AgentRef) error {
	if agent.Name == "" {
		return fmt.Errorf("handoff: agent name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.agents[agent.Name]; exists {
		return fmt.Errorf("handoff: agent %q already registered", agent.Name)
	}
	m.agents[agent.Name] = agent
	return nil
}

// Unregister removes an agent by name. Returns true if the agent existed.
func (m *HandoffManager) Unregister(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, existed := m.agents[name]
	delete(m.agents, name)
	return existed
}

// Delegate dispatches work to the named agent, enforcing delegation depth and timeout.
// It requires a DelegateFunc to be set in the Config; returns ErrNoDelegateFunc otherwise.
func (m *HandoffManager) Delegate(ctx context.Context, agentName string, req HandoffRequest) (*HandoffResult, error) {
	if m.config.Delegate == nil {
		return nil, ErrNoDelegateFunc
	}

	// Enforce max delegation depth.
	depth := DelegationDepth(ctx)
	if depth >= m.config.MaxDelegationDepth {
		return nil, ErrDelegationDepth
	}

	m.mu.RLock()
	agent, ok := m.agents[agentName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, agentName)
	}

	// Apply timeout — use request-level override, fall back to manager default.
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = m.config.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Propagate depth so downstream delegates can enforce their own limits.
	ctx = WithDelegationDepth(ctx, depth+1)

	start := time.Now()
	result, err := m.config.Delegate(ctx, agent, req)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			return &HandoffResult{
				AgentName: agentName,
				Status:    "timeout",
				Result:    "delegation timed out",
				Duration:  elapsed,
			}, ErrDelegateTimeout
		}
		return nil, err
	}

	result.AgentName = agentName
	result.Duration = elapsed

	// Record cost when a tracker is configured and the result carried token counts.
	if m.config.CostTracker != nil && result.TokensUsed > 0 {
		m.config.CostTracker.Record(finops.UsageEntry{
			ToolName:     "handoff:" + agentName,
			Category:     "handoff",
			InputTokens:  int(result.TokensUsed / 2), // rough equal split
			OutputTokens: int(result.TokensUsed / 2),
			Duration:     elapsed,
			Timestamp:    time.Now(),
		})
	}

	return result, nil
}

// ListAgents returns all registered agents sorted by name.
func (m *HandoffManager) ListAgents() []AgentRef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]AgentRef, 0, len(m.agents))
	for _, a := range m.agents {
		agents = append(agents, a)
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents
}

// FindAgent returns the first registered agent that has the given skill.
func (m *HandoffManager) FindAgent(skill string) (AgentRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.agents {
		if slices.Contains(a.Skills, skill) {
			return a, true
		}
	}
	return AgentRef{}, false
}
