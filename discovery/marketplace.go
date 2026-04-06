package discovery

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Sentinel errors returned by Marketplace methods.
var (
	ErrAgentNotFound    = errors.New("marketplace: agent not found")
	ErrAgentExists      = errors.New("marketplace: agent already registered")
	ErrNoMatch          = errors.New("marketplace: no matching agent found")
	ErrEmptyID          = errors.New("marketplace: agent ID must not be empty")
	ErrEmptyName        = errors.New("marketplace: agent name must not be empty")
	ErrInvalidTrustScore = errors.New("marketplace: trust score must be between 0.0 and 1.0")
)

// AgentEntry represents a discoverable agent in the marketplace.
type AgentEntry struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities,omitempty"` // e.g., "code-review", "research", "deploy"
	Protocol     string    `json:"protocol"`               // "mcp", "a2a", "openai"
	Endpoint     string    `json:"endpoint"`               // How to reach the agent
	TrustScore   float64   `json:"trust_score"`            // 0.0-1.0 based on history
	CostPerTask  float64   `json:"cost_per_task"`          // Average cost
	SuccessRate  float64   `json:"success_rate"`           // Task completion rate
	LastSeen     time.Time `json:"last_seen"`
}

// MarketplaceStats holds aggregate statistics about the marketplace.
type MarketplaceStats struct {
	TotalAgents      int                `json:"total_agents"`
	ByProtocol       map[string]int     `json:"by_protocol"`
	ByCapability     map[string]int     `json:"by_capability"`
	AvgTrustScore    float64            `json:"avg_trust_score"`
	AvgCostPerTask   float64            `json:"avg_cost_per_task"`
	AvgSuccessRate   float64            `json:"avg_success_rate"`
}

// ratingAccumulator tracks cumulative task outcomes for trust score computation.
type ratingAccumulator struct {
	totalTasks    int
	successCount  int
	totalCost     float64
}

// Marketplace indexes and searches for agents by capability.
type Marketplace struct {
	agents  map[string]*AgentEntry
	ratings map[string]*ratingAccumulator
	mu      sync.RWMutex
}

// NewMarketplace creates an empty Marketplace ready for agent registration.
func NewMarketplace() *Marketplace {
	return &Marketplace{
		agents:  make(map[string]*AgentEntry),
		ratings: make(map[string]*ratingAccumulator),
	}
}

// Register adds an agent entry to the marketplace. Returns ErrAgentExists if
// an agent with the same ID is already registered, ErrEmptyID if the ID is
// empty, or ErrEmptyName if the name is empty.
func (m *Marketplace) Register(entry AgentEntry) error {
	if entry.ID == "" {
		return ErrEmptyID
	}
	if entry.Name == "" {
		return ErrEmptyName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[entry.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAgentExists, entry.ID)
	}

	// Clamp trust score to [0,1].
	entry.TrustScore = clamp(entry.TrustScore, 0.0, 1.0)

	// Clamp success rate to [0,1].
	entry.SuccessRate = clamp(entry.SuccessRate, 0.0, 1.0)

	// Set LastSeen if not provided.
	if entry.LastSeen.IsZero() {
		entry.LastSeen = time.Now()
	}

	// Store a copy to prevent external mutation.
	stored := entry
	stored.Capabilities = copyStrings(entry.Capabilities)
	m.agents[entry.ID] = &stored
	m.ratings[entry.ID] = &ratingAccumulator{}

	return nil
}

// Deregister removes an agent from the marketplace by ID. Returns
// ErrAgentNotFound if the agent does not exist.
func (m *Marketplace) Deregister(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[id]; !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, id)
	}

	delete(m.agents, id)
	delete(m.ratings, id)
	return nil
}

// Search returns agents matching the query string and/or capabilities. The
// query is matched case-insensitively against agent name and description.
// Capabilities are matched as a subset — an agent must have ALL requested
// capabilities to be included. If both query and capabilities are empty, all
// agents are returned. Results are sorted by trust score descending.
func (m *Marketplace) Search(query string, capabilities []string) []AgentEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	var results []AgentEntry

	for _, agent := range m.agents {
		if !matchesQuery(agent, query) {
			continue
		}
		if !hasAllCapabilities(agent, capabilities) {
			continue
		}
		// Return a copy.
		entry := *agent
		entry.Capabilities = copyStrings(agent.Capabilities)
		results = append(results, entry)
	}

	// Sort by trust score descending, then by name for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		if results[i].TrustScore != results[j].TrustScore {
			return results[i].TrustScore > results[j].TrustScore
		}
		return results[i].Name < results[j].Name
	})

	return results
}

// Rate records a task outcome for an agent, updating its trust score, success
// rate, and average cost. Returns ErrAgentNotFound if the agent does not exist.
func (m *Marketplace) Rate(id string, success bool, cost float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[id]
	if !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, id)
	}

	acc, exists := m.ratings[id]
	if !exists {
		acc = &ratingAccumulator{}
		m.ratings[id] = acc
	}

	acc.totalTasks++
	if success {
		acc.successCount++
	}
	acc.totalCost += cost

	// Recompute derived metrics.
	agent.SuccessRate = float64(acc.successCount) / float64(acc.totalTasks)
	agent.CostPerTask = acc.totalCost / float64(acc.totalTasks)

	// Trust score is a weighted blend of success rate. We use an exponential
	// moving average that gives more weight as more tasks accumulate, capping
	// the confidence factor at 1.0 after 10 tasks.
	confidence := math.Min(float64(acc.totalTasks)/10.0, 1.0)
	agent.TrustScore = agent.SuccessRate * confidence

	agent.LastSeen = time.Now()

	return nil
}

// Stats returns aggregate statistics about all registered agents.
func (m *Marketplace) Stats() MarketplaceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := MarketplaceStats{
		TotalAgents:  len(m.agents),
		ByProtocol:   make(map[string]int),
		ByCapability: make(map[string]int),
	}

	if stats.TotalAgents == 0 {
		return stats
	}

	var totalTrust, totalCost, totalSuccess float64
	for _, agent := range m.agents {
		totalTrust += agent.TrustScore
		totalCost += agent.CostPerTask
		totalSuccess += agent.SuccessRate

		if agent.Protocol != "" {
			stats.ByProtocol[agent.Protocol]++
		}
		for _, cap := range agent.Capabilities {
			stats.ByCapability[cap]++
		}
	}

	n := float64(stats.TotalAgents)
	stats.AvgTrustScore = totalTrust / n
	stats.AvgCostPerTask = totalCost / n
	stats.AvgSuccessRate = totalSuccess / n

	return stats
}

// BestMatch returns the agent with the highest combined score for the given
// capabilities. The score is computed as: trust_score * capability_overlap,
// where capability_overlap is the fraction of requested capabilities the agent
// supports. Returns ErrNoMatch if no agent has any overlap.
func (m *Marketplace) BestMatch(capabilities []string) (*AgentEntry, error) {
	if len(capabilities) == 0 {
		return nil, ErrNoMatch
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *AgentEntry
	bestScore := -1.0

	for _, agent := range m.agents {
		overlap := capabilityOverlap(agent, capabilities)
		if overlap == 0 {
			continue
		}

		score := agent.TrustScore * overlap
		if score > bestScore || (score == bestScore && best != nil && agent.Name < best.Name) {
			cp := *agent
			cp.Capabilities = copyStrings(agent.Capabilities)
			best = &cp
			bestScore = score
		}
	}

	if best == nil {
		return nil, ErrNoMatch
	}

	return best, nil
}

// --- helpers ---

// matchesQuery checks if the agent matches the lowered query string against
// its name or description. An empty query matches everything.
func matchesQuery(agent *AgentEntry, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(agent.Name), query) ||
		strings.Contains(strings.ToLower(agent.Description), query)
}

// hasAllCapabilities checks if the agent has every capability in the required
// list. An empty required list matches everything.
func hasAllCapabilities(agent *AgentEntry, required []string) bool {
	if len(required) == 0 {
		return true
	}
	capSet := make(map[string]struct{}, len(agent.Capabilities))
	for _, c := range agent.Capabilities {
		capSet[c] = struct{}{}
	}
	for _, r := range required {
		if _, ok := capSet[r]; !ok {
			return false
		}
	}
	return true
}

// capabilityOverlap returns the fraction of requested capabilities that the
// agent supports, as a value between 0.0 and 1.0.
func capabilityOverlap(agent *AgentEntry, requested []string) float64 {
	if len(requested) == 0 {
		return 0
	}
	capSet := make(map[string]struct{}, len(agent.Capabilities))
	for _, c := range agent.Capabilities {
		capSet[c] = struct{}{}
	}
	matches := 0
	for _, r := range requested {
		if _, ok := capSet[r]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(requested))
}

// copyStrings returns a shallow copy of a string slice.
func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

// clamp constrains v to the range [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
