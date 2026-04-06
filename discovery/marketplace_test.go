package discovery

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

// sampleAgent returns a minimal AgentEntry for use in tests.
func sampleAgent(id, name string, capabilities ...string) AgentEntry {
	return AgentEntry{
		ID:           id,
		Name:         name,
		Description:  "A test agent for " + name,
		Capabilities: capabilities,
		Protocol:     "mcp",
		Endpoint:     "stdio://" + id,
		TrustScore:   0.8,
		CostPerTask:  0.05,
		SuccessRate:  0.9,
		LastSeen:     time.Now(),
	}
}

// --- Register + Search ---

func TestMarketplace_RegisterSearch(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	// Register two agents with different capabilities.
	err := m.Register(sampleAgent("agent-1", "Code Reviewer", "code-review", "lint"))
	if err != nil {
		t.Fatalf("Register agent-1: %v", err)
	}
	err = m.Register(sampleAgent("agent-2", "Researcher", "research", "summarize"))
	if err != nil {
		t.Fatalf("Register agent-2: %v", err)
	}

	// Search with no filters returns all agents.
	all := m.Search("", nil)
	if len(all) != 2 {
		t.Fatalf("Search all: got %d agents, want 2", len(all))
	}

	// Search by query — matches name.
	byName := m.Search("reviewer", nil)
	if len(byName) != 1 {
		t.Fatalf("Search by name: got %d agents, want 1", len(byName))
	}
	if byName[0].ID != "agent-1" {
		t.Errorf("Search by name: got ID %q, want %q", byName[0].ID, "agent-1")
	}

	// Search by query — matches description.
	byDesc := m.Search("researcher", nil)
	if len(byDesc) != 1 {
		t.Fatalf("Search by description: got %d agents, want 1", len(byDesc))
	}
	if byDesc[0].ID != "agent-2" {
		t.Errorf("Search by description: got ID %q, want %q", byDesc[0].ID, "agent-2")
	}

	// Search by capability.
	byCap := m.Search("", []string{"code-review"})
	if len(byCap) != 1 {
		t.Fatalf("Search by capability: got %d agents, want 1", len(byCap))
	}
	if byCap[0].ID != "agent-1" {
		t.Errorf("Search by capability: got ID %q, want %q", byCap[0].ID, "agent-1")
	}

	// Search by multiple capabilities — must have ALL.
	byMultiCap := m.Search("", []string{"research", "summarize"})
	if len(byMultiCap) != 1 {
		t.Fatalf("Search by multi-cap: got %d agents, want 1", len(byMultiCap))
	}
	if byMultiCap[0].ID != "agent-2" {
		t.Errorf("Search by multi-cap: got ID %q, want %q", byMultiCap[0].ID, "agent-2")
	}

	// Search by capability that no agent has.
	noMatch := m.Search("", []string{"deploy"})
	if len(noMatch) != 0 {
		t.Errorf("Search for missing capability: got %d agents, want 0", len(noMatch))
	}
}

func TestMarketplace_Register_DuplicateID(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	if err := m.Register(sampleAgent("dup", "First")); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := m.Register(sampleAgent("dup", "Second"))
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
	if !errors.Is(err, ErrAgentExists) {
		t.Errorf("error = %v, want ErrAgentExists", err)
	}
}

func TestMarketplace_Register_EmptyID(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	err := m.Register(AgentEntry{Name: "No ID"})
	if !errors.Is(err, ErrEmptyID) {
		t.Errorf("error = %v, want ErrEmptyID", err)
	}
}

func TestMarketplace_Register_EmptyName(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	err := m.Register(AgentEntry{ID: "no-name"})
	if !errors.Is(err, ErrEmptyName) {
		t.Errorf("error = %v, want ErrEmptyName", err)
	}
}

func TestMarketplace_Register_ClampsTrustScore(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	// Trust score above 1.0 should be clamped.
	entry := sampleAgent("high", "High Trust")
	entry.TrustScore = 1.5
	if err := m.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}

	results := m.Search("high trust", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TrustScore != 1.0 {
		t.Errorf("TrustScore = %f, want 1.0", results[0].TrustScore)
	}
}

func TestMarketplace_Search_CaseInsensitive(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	if err := m.Register(sampleAgent("ci", "Alpha Bot")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	results := m.Search("ALPHA", nil)
	if len(results) != 1 {
		t.Errorf("case-insensitive search: got %d results, want 1", len(results))
	}
}

func TestMarketplace_Search_SortedByTrustScore(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	low := sampleAgent("low", "Low Trust")
	low.TrustScore = 0.3
	high := sampleAgent("high", "High Trust")
	high.TrustScore = 0.9
	mid := sampleAgent("mid", "Mid Trust")
	mid.TrustScore = 0.6

	for _, a := range []AgentEntry{low, high, mid} {
		if err := m.Register(a); err != nil {
			t.Fatalf("Register %s: %v", a.ID, err)
		}
	}

	results := m.Search("", nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].ID != "high" {
		t.Errorf("results[0].ID = %q, want %q", results[0].ID, "high")
	}
	if results[1].ID != "mid" {
		t.Errorf("results[1].ID = %q, want %q", results[1].ID, "mid")
	}
	if results[2].ID != "low" {
		t.Errorf("results[2].ID = %q, want %q", results[2].ID, "low")
	}
}

// --- Deregister ---

func TestMarketplace_Deregister(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	if err := m.Register(sampleAgent("rm-me", "Removable")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Deregister should succeed.
	if err := m.Deregister("rm-me"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	// Search should return empty.
	results := m.Search("", nil)
	if len(results) != 0 {
		t.Errorf("after deregister: got %d agents, want 0", len(results))
	}

	// Deregistering again should error.
	err := m.Deregister("rm-me")
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("error = %v, want ErrAgentNotFound", err)
	}
}

func TestMarketplace_Deregister_NotFound(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	err := m.Deregister("nonexistent")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("error = %v, want ErrAgentNotFound", err)
	}
}

// --- Rate ---

func TestMarketplace_Rate(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	entry := sampleAgent("rated", "Rated Agent")
	entry.TrustScore = 0.5
	entry.CostPerTask = 0.0
	entry.SuccessRate = 0.0
	if err := m.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Rate with 3 successes.
	for i := range 3 {
		if err := m.Rate("rated", true, 0.10); err != nil {
			t.Fatalf("Rate success %d: %v", i, err)
		}
	}

	// Rate with 1 failure.
	if err := m.Rate("rated", false, 0.05); err != nil {
		t.Fatalf("Rate failure: %v", err)
	}

	// Verify metrics via Search.
	results := m.Search("rated", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	agent := results[0]

	// Success rate: 3/4 = 0.75.
	if math.Abs(agent.SuccessRate-0.75) > 0.001 {
		t.Errorf("SuccessRate = %f, want 0.75", agent.SuccessRate)
	}

	// Cost per task: (0.10*3 + 0.05) / 4 = 0.35/4 = 0.0875.
	if math.Abs(agent.CostPerTask-0.0875) > 0.001 {
		t.Errorf("CostPerTask = %f, want 0.0875", agent.CostPerTask)
	}

	// Trust score: successRate * confidence, where confidence = min(4/10, 1.0) = 0.4.
	// So trust = 0.75 * 0.4 = 0.3.
	expectedTrust := 0.75 * 0.4
	if math.Abs(agent.TrustScore-expectedTrust) > 0.001 {
		t.Errorf("TrustScore = %f, want %f", agent.TrustScore, expectedTrust)
	}
}

func TestMarketplace_Rate_FullConfidence(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	entry := sampleAgent("full", "Full Confidence")
	entry.TrustScore = 0.0
	if err := m.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Rate 10 successes — confidence should reach 1.0.
	for i := range 10 {
		if err := m.Rate("full", true, 0.01); err != nil {
			t.Fatalf("Rate %d: %v", i, err)
		}
	}

	results := m.Search("full confidence", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Trust = 1.0 * 1.0 = 1.0 (all successes, full confidence).
	if math.Abs(results[0].TrustScore-1.0) > 0.001 {
		t.Errorf("TrustScore = %f, want 1.0", results[0].TrustScore)
	}
}

func TestMarketplace_Rate_NotFound(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	err := m.Rate("ghost", true, 0.01)
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("error = %v, want ErrAgentNotFound", err)
	}
}

// --- BestMatch ---

func TestMarketplace_BestMatch(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	// Agent A: high trust, partial capability overlap.
	a := sampleAgent("a", "Agent A", "code-review", "lint")
	a.TrustScore = 0.9

	// Agent B: lower trust, full capability overlap.
	b := sampleAgent("b", "Agent B", "code-review", "lint", "deploy")
	b.TrustScore = 0.7

	// Agent C: high trust, no overlap.
	c := sampleAgent("c", "Agent C", "research", "summarize")
	c.TrustScore = 0.95

	for _, agent := range []AgentEntry{a, b, c} {
		if err := m.Register(agent); err != nil {
			t.Fatalf("Register %s: %v", agent.ID, err)
		}
	}

	// Request code-review + lint + deploy.
	best, err := m.BestMatch([]string{"code-review", "lint", "deploy"})
	if err != nil {
		t.Fatalf("BestMatch: %v", err)
	}

	// Agent A: overlap = 2/3 = 0.667, score = 0.9 * 0.667 = 0.6
	// Agent B: overlap = 3/3 = 1.0,   score = 0.7 * 1.0   = 0.7
	// Agent C: overlap = 0/3 = 0.0,   excluded
	// Best = Agent B.
	if best.ID != "b" {
		t.Errorf("BestMatch ID = %q, want %q", best.ID, "b")
	}
}

func TestMarketplace_BestMatch_TieBreaksByName(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	// Two agents with identical trust and capability overlap.
	x := sampleAgent("x", "Bravo", "deploy")
	x.TrustScore = 0.8
	y := sampleAgent("y", "Alpha", "deploy")
	y.TrustScore = 0.8

	for _, agent := range []AgentEntry{x, y} {
		if err := m.Register(agent); err != nil {
			t.Fatalf("Register %s: %v", agent.ID, err)
		}
	}

	best, err := m.BestMatch([]string{"deploy"})
	if err != nil {
		t.Fatalf("BestMatch: %v", err)
	}

	// Same score, so tie-break by name — "Alpha" < "Bravo".
	if best.ID != "y" {
		t.Errorf("BestMatch ID = %q, want %q (tie-break by name)", best.ID, "y")
	}
}

func TestMarketplace_BestMatch_NoOverlap(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	if err := m.Register(sampleAgent("a", "Agent A", "code-review")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := m.BestMatch([]string{"deploy"})
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("error = %v, want ErrNoMatch", err)
	}
}

func TestMarketplace_BestMatch_EmptyCapabilities(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	_, err := m.BestMatch(nil)
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("error = %v, want ErrNoMatch", err)
	}

	_, err = m.BestMatch([]string{})
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("error = %v, want ErrNoMatch for empty slice", err)
	}
}

// --- Stats ---

func TestMarketplace_Stats(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	a := sampleAgent("a", "Agent A", "code-review", "lint")
	a.Protocol = "mcp"
	a.TrustScore = 0.8
	a.CostPerTask = 0.10
	a.SuccessRate = 0.9

	b := sampleAgent("b", "Agent B", "research")
	b.Protocol = "a2a"
	b.TrustScore = 0.6
	b.CostPerTask = 0.20
	b.SuccessRate = 0.7

	c := sampleAgent("c", "Agent C", "code-review", "deploy")
	c.Protocol = "mcp"
	c.TrustScore = 0.7
	c.CostPerTask = 0.15
	c.SuccessRate = 0.8

	for _, agent := range []AgentEntry{a, b, c} {
		if err := m.Register(agent); err != nil {
			t.Fatalf("Register %s: %v", agent.ID, err)
		}
	}

	stats := m.Stats()

	if stats.TotalAgents != 3 {
		t.Errorf("TotalAgents = %d, want 3", stats.TotalAgents)
	}

	// Protocol counts.
	if stats.ByProtocol["mcp"] != 2 {
		t.Errorf("ByProtocol[mcp] = %d, want 2", stats.ByProtocol["mcp"])
	}
	if stats.ByProtocol["a2a"] != 1 {
		t.Errorf("ByProtocol[a2a] = %d, want 1", stats.ByProtocol["a2a"])
	}

	// Capability counts.
	if stats.ByCapability["code-review"] != 2 {
		t.Errorf("ByCapability[code-review] = %d, want 2", stats.ByCapability["code-review"])
	}
	if stats.ByCapability["lint"] != 1 {
		t.Errorf("ByCapability[lint] = %d, want 1", stats.ByCapability["lint"])
	}
	if stats.ByCapability["research"] != 1 {
		t.Errorf("ByCapability[research] = %d, want 1", stats.ByCapability["research"])
	}
	if stats.ByCapability["deploy"] != 1 {
		t.Errorf("ByCapability[deploy] = %d, want 1", stats.ByCapability["deploy"])
	}

	// Averages: (0.8+0.6+0.7)/3 = 0.7, (0.10+0.20+0.15)/3 = 0.15, (0.9+0.7+0.8)/3 = 0.8.
	if math.Abs(stats.AvgTrustScore-0.7) > 0.001 {
		t.Errorf("AvgTrustScore = %f, want 0.7", stats.AvgTrustScore)
	}
	if math.Abs(stats.AvgCostPerTask-0.15) > 0.001 {
		t.Errorf("AvgCostPerTask = %f, want 0.15", stats.AvgCostPerTask)
	}
	if math.Abs(stats.AvgSuccessRate-0.8) > 0.001 {
		t.Errorf("AvgSuccessRate = %f, want 0.8", stats.AvgSuccessRate)
	}
}

func TestMarketplace_Stats_Empty(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	stats := m.Stats()
	if stats.TotalAgents != 0 {
		t.Errorf("TotalAgents = %d, want 0", stats.TotalAgents)
	}
	if stats.AvgTrustScore != 0 {
		t.Errorf("AvgTrustScore = %f, want 0", stats.AvgTrustScore)
	}
}

// --- Concurrent ---

func TestMarketplace_Concurrent(t *testing.T) {
	t.Parallel()
	m := NewMarketplace()

	const numWorkers = 20
	const opsPerWorker = 50

	// Pre-register some agents so concurrent operations have targets.
	for i := range 10 {
		id := fmt.Sprintf("seed-%d", i)
		if err := m.Register(sampleAgent(id, "Seed "+id, "cap-a", "cap-b")); err != nil {
			t.Fatalf("seed Register: %v", err)
		}
	}

	var wg sync.WaitGroup

	// Concurrent registers.
	for w := range numWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range opsPerWorker {
				id := fmt.Sprintf("w%d-r%d", worker, i)
				_ = m.Register(sampleAgent(id, "Worker "+id, "dynamic"))
			}
		}(w)
	}

	// Concurrent searches.
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range opsPerWorker {
				_ = m.Search("", []string{"cap-a"})
				_ = m.Search("seed", nil)
			}
		}()
	}

	// Concurrent ratings on seed agents.
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range opsPerWorker {
				id := fmt.Sprintf("seed-%d", i%10)
				_ = m.Rate(id, i%3 != 0, 0.01)
			}
		}()
	}

	// Concurrent stats.
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range opsPerWorker {
				_ = m.Stats()
			}
		}()
	}

	// Concurrent BestMatch.
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range opsPerWorker {
				_, _ = m.BestMatch([]string{"cap-a", "cap-b"})
			}
		}()
	}

	// Concurrent deregisters — only on dynamically registered agents.
	for w := range numWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range opsPerWorker {
				id := fmt.Sprintf("w%d-r%d", worker, i)
				_ = m.Deregister(id)
			}
		}(w)
	}

	wg.Wait()

	// If we got here without a race detector panic, the test passes.
	// Verify seed agents still exist (they were not deregistered).
	stats := m.Stats()
	if stats.TotalAgents < 10 {
		t.Errorf("expected at least 10 seed agents remaining, got %d", stats.TotalAgents)
	}
}
