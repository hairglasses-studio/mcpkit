// Package rdcycle orchestrates the R&D cycle by composing research and roadmap tools.
//
// It provides an ArtifactStore for persisting cycle step outputs, and a set of
// MCP tools for scanning the ecosystem, planning next work, verifying builds,
// and listing stored artifacts. Designed to be driven by an autonomous loop
// (e.g., the Ralph Loop pattern).
package rdcycle

import (
	"sync"
)

// CycleConfig configures the R&D cycle orchestrator.
type CycleConfig struct {
	RoadmapPath string   // Path to roadmap.json
	GitRoot     string   // Root of the git repository
	ScanRepos   []string // GitHub repos to monitor (owner/repo)
	DateRange   string   // ISO 8601 since date for scans
}

// Artifact represents a piece of work output (code diff, test result, build output).
type Artifact struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // "scan", "plan", "verify", "code", "test"
	Content   map[string]any `json:"content"`
	CreatedAt string         `json:"created_at"`
}

// ArtifactStore persists artifacts from R&D cycle steps.
type ArtifactStore interface {
	Save(artifact Artifact) error
	Get(id string) (Artifact, bool)
	List(artifactType string) []Artifact
}

// InMemoryArtifactStore is a thread-safe in-memory implementation of ArtifactStore.
type InMemoryArtifactStore struct {
	mu        sync.RWMutex
	artifacts map[string]Artifact
}

// NewInMemoryArtifactStore creates a new InMemoryArtifactStore.
func NewInMemoryArtifactStore() *InMemoryArtifactStore {
	return &InMemoryArtifactStore{
		artifacts: make(map[string]Artifact),
	}
}

// Save stores an artifact by ID, overwriting any existing artifact with the same ID.
func (s *InMemoryArtifactStore) Save(artifact Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts[artifact.ID] = artifact
	return nil
}

// Get retrieves an artifact by ID. Returns false if not found.
func (s *InMemoryArtifactStore) Get(id string) (Artifact, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.artifacts[id]
	return a, ok
}

// List returns all artifacts matching the given type. If artifactType is empty,
// all artifacts are returned. Results are not guaranteed to be in insertion order.
func (s *InMemoryArtifactStore) List(artifactType string) []Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Artifact
	for _, a := range s.artifacts {
		if artifactType == "" || a.Type == artifactType {
			result = append(result, a)
		}
	}
	return result
}
