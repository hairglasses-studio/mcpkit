package rdcycle

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/roadmap"
)

// RoadmapSource fetches candidate tasks from the next roadmap phase.
type RoadmapSource struct {
	RoadmapPath string
}

// NewRoadmapSource creates a RoadmapSource.
func NewRoadmapSource(roadmapPath string) *RoadmapSource {
	return &RoadmapSource{RoadmapPath: roadmapPath}
}

// Fetch returns candidate tasks from the next incomplete roadmap phase.
func (rs *RoadmapSource) Fetch(_ context.Context) ([]CandidateTask, error) {
	rm, err := roadmap.LoadRoadmap(rs.RoadmapPath)
	if err != nil {
		return nil, err
	}

	phase := roadmap.NextPhase(rm)
	if phase == nil {
		return nil, nil
	}

	ready := roadmap.ReadyItems(phase)
	var candidates []CandidateTask
	for i, item := range ready {
		complexity := "moderate"
		if item.Priority == "high" {
			complexity = "complex"
		} else if item.Priority == "low" {
			complexity = "simple"
		}
		candidates = append(candidates, CandidateTask{
			ID:          item.ID,
			Description: item.Description,
			Source:      "roadmap",
			Priority:    i + 10,
			DependsOn:   item.DependsOn,
			Complexity:  complexity,
		})
	}
	return candidates, nil
}
