// Package roadmap provides machine-readable roadmap management types and tools
// for tracking project phases, work items, and gap analysis.
package roadmap

// ItemStatus represents the current state of a work item.
type ItemStatus string

const (
	ItemStatusPlanned  ItemStatus = "planned"
	ItemStatusActive   ItemStatus = "active"
	ItemStatusComplete ItemStatus = "complete"
)

// PhaseStatus represents the current state of a phase.
type PhaseStatus string

const (
	PhaseStatusPlanned  PhaseStatus = "planned"
	PhaseStatusActive   PhaseStatus = "active"
	PhaseStatusComplete PhaseStatus = "complete"
)

// Roadmap is the top-level machine-readable roadmap structure.
type Roadmap struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Phases      []Phase `json:"phases"`
	Tiers       []Tier  `json:"tiers,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

// Phase is a named group of work items with an ordered identifier.
type Phase struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Status PhaseStatus `json:"status"`
	Items  []WorkItem  `json:"items"`
}

// Tier represents a dependency-layer grouping of work items.
type Tier struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []WorkItem `json:"items"`
}

// WorkItem is a single unit of trackable work.
type WorkItem struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Package     string     `json:"package,omitempty"`
	Status      ItemStatus `json:"status"`
	DependsOn   []string   `json:"depends_on,omitempty"`
	Priority    string     `json:"priority,omitempty"` // high/medium/low
}

// XML tag constants used in RenderMarkdown output for agent-searchable content.
const (
	TagRoadmapPhase = "roadmap-phase"
	TagRoadmapItem  = "roadmap-item"
	TagRoadmapTier  = "roadmap-tier"
)
