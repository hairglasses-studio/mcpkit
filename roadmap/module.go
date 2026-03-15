package roadmap

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Config configures the roadmap module.
type Config struct {
	// RoadmapPath is the path to the JSON roadmap file (default "roadmap.json").
	RoadmapPath string
}

// Module implements registry.ToolModule for roadmap management tools.
type Module struct {
	roadmapPath string
}

// NewModule creates a roadmap module with the given configuration.
func NewModule(cfg ...Config) *Module {
	m := &Module{
		roadmapPath: "roadmap.json",
	}
	if len(cfg) > 0 && cfg[0].RoadmapPath != "" {
		m.roadmapPath = cfg[0].RoadmapPath
	}
	return m
}

// Name returns the module name.
func (m *Module) Name() string { return "roadmap" }

// Description returns the module description.
func (m *Module) Description() string {
	return "Machine-readable roadmap management tools for tracking phases, items, and gaps"
}

// Tools returns all roadmap tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	tools := []registry.ToolDefinition{
		m.readTool(),
		m.updateTool(),
		m.gapsTool(),
		m.nextPhaseTool(),
	}

	for i := range tools {
		tools[i].Category = "roadmap"
		tools[i].Timeout = 10 * time.Second
		tools[i].Complexity = registry.ComplexitySimple
	}

	return tools
}

// --- roadmap_read ---

// ReadInput is the input for the roadmap_read tool.
type ReadInput struct {
	Path string `json:"path,omitempty" jsonschema:"description=Path to the roadmap JSON file. Defaults to the module-configured path."`
}

// ReadOutput is the output of the roadmap_read tool.
type ReadOutput struct {
	Roadmap  *Roadmap `json:"roadmap"`
	Markdown string   `json:"markdown"`
}

func (m *Module) readTool() registry.ToolDefinition {
	desc := "Read the roadmap JSON file and return the structured roadmap with rendered markdown. " +
		"Use this to understand the current state of the project roadmap, phases, and work items."

	return handler.TypedHandler[ReadInput, ReadOutput](
		"roadmap_read",
		desc,
		m.handleRead,
	)
}

func (m *Module) handleRead(ctx context.Context, input ReadInput) (ReadOutput, error) {
	path := m.roadmapPath
	if input.Path != "" {
		path = input.Path
	}

	rm, err := LoadRoadmap(path)
	if err != nil {
		return ReadOutput{}, fmt.Errorf("roadmap_read: %w", err)
	}

	return ReadOutput{
		Roadmap:  rm,
		Markdown: RenderMarkdown(rm),
	}, nil
}

// --- roadmap_update ---

// UpdateInput is the input for the roadmap_update tool.
type UpdateInput struct {
	ItemID string     `json:"item_id" jsonschema:"required,description=The ID of the work item to update (e.g. '20A')"`
	Status ItemStatus `json:"status" jsonschema:"required,description=New status for the item: planned/active/complete"`
	Path   string     `json:"path,omitempty" jsonschema:"description=Path to the roadmap JSON file. Defaults to the module-configured path."`
}

// UpdateOutput is the output of the roadmap_update tool.
type UpdateOutput struct {
	Updated bool       `json:"updated"`
	ItemID  string     `json:"item_id"`
	Status  ItemStatus `json:"status"`
	Message string     `json:"message"`
}

func (m *Module) updateTool() registry.ToolDefinition {
	desc := "Update a work item's status by ID and save the roadmap back to disk. " +
		"Use this to mark items as active or complete as work progresses."

	td := handler.TypedHandler[UpdateInput, UpdateOutput](
		"roadmap_update",
		desc,
		m.handleUpdate,
	)
	td.IsWrite = true
	return td
}

func (m *Module) handleUpdate(ctx context.Context, input UpdateInput) (UpdateOutput, error) {
	if input.ItemID == "" {
		return UpdateOutput{}, fmt.Errorf("item_id is required")
	}

	switch input.Status {
	case ItemStatusPlanned, ItemStatusActive, ItemStatusComplete:
		// valid
	default:
		return UpdateOutput{}, fmt.Errorf("invalid status %q: must be planned, active, or complete", input.Status)
	}

	path := m.roadmapPath
	if input.Path != "" {
		path = input.Path
	}

	rm, err := LoadRoadmap(path)
	if err != nil {
		return UpdateOutput{}, fmt.Errorf("roadmap_update: %w", err)
	}

	updated := false
	for pi := range rm.Phases {
		for ii := range rm.Phases[pi].Items {
			if rm.Phases[pi].Items[ii].ID == input.ItemID {
				rm.Phases[pi].Items[ii].Status = input.Status
				updated = true
			}
		}
	}
	for ti := range rm.Tiers {
		for ii := range rm.Tiers[ti].Items {
			if rm.Tiers[ti].Items[ii].ID == input.ItemID {
				rm.Tiers[ti].Items[ii].Status = input.Status
				updated = true
			}
		}
	}

	if !updated {
		return UpdateOutput{
			Updated: false,
			ItemID:  input.ItemID,
			Status:  input.Status,
			Message: fmt.Sprintf("item %q not found in roadmap", input.ItemID),
		}, nil
	}

	if err := SaveRoadmap(path, rm); err != nil {
		return UpdateOutput{}, fmt.Errorf("roadmap_update: save: %w", err)
	}

	return UpdateOutput{
		Updated: true,
		ItemID:  input.ItemID,
		Status:  input.Status,
		Message: fmt.Sprintf("item %q updated to %q", input.ItemID, input.Status),
	}, nil
}

// --- roadmap_gaps ---

// GapsInput is the input for the roadmap_gaps tool.
type GapsInput struct {
	Path string `json:"path,omitempty" jsonschema:"description=Path to the roadmap JSON file. Defaults to the module-configured path."`
}

// GapsOutput is the output of the roadmap_gaps tool.
type GapsOutput struct {
	Gaps  []WorkItem `json:"gaps"`
	Count int        `json:"count"`
}

func (m *Module) gapsTool() registry.ToolDefinition {
	desc := "Run gap analysis on the roadmap and return all planned (not yet started) work items. " +
		"Use this to identify what work remains before starting new phases."

	return handler.TypedHandler[GapsInput, GapsOutput](
		"roadmap_gaps",
		desc,
		m.handleGaps,
	)
}

func (m *Module) handleGaps(ctx context.Context, input GapsInput) (GapsOutput, error) {
	path := m.roadmapPath
	if input.Path != "" {
		path = input.Path
	}

	rm, err := LoadRoadmap(path)
	if err != nil {
		return GapsOutput{}, fmt.Errorf("roadmap_gaps: %w", err)
	}

	gaps := GapAnalysis(rm)
	return GapsOutput{
		Gaps:  gaps,
		Count: len(gaps),
	}, nil
}

// --- roadmap_next_phase ---

// NextPhaseInput is the input for the roadmap_next_phase tool.
type NextPhaseInput struct {
	Path string `json:"path,omitempty" jsonschema:"description=Path to the roadmap JSON file. Defaults to the module-configured path."`
}

// NextPhaseOutput is the output of the roadmap_next_phase tool.
type NextPhaseOutput struct {
	Phase      *Phase     `json:"phase"`
	ReadyItems []WorkItem `json:"ready_items"`
	Found      bool       `json:"found"`
}

func (m *Module) nextPhaseTool() registry.ToolDefinition {
	desc := "Return the next incomplete phase and its ready-to-start work items. " +
		"Use this to understand what to work on next and which items are unblocked."

	return handler.TypedHandler[NextPhaseInput, NextPhaseOutput](
		"roadmap_next_phase",
		desc,
		m.handleNextPhase,
	)
}

func (m *Module) handleNextPhase(ctx context.Context, input NextPhaseInput) (NextPhaseOutput, error) {
	path := m.roadmapPath
	if input.Path != "" {
		path = input.Path
	}

	rm, err := LoadRoadmap(path)
	if err != nil {
		return NextPhaseOutput{}, fmt.Errorf("roadmap_next_phase: %w", err)
	}

	phase := NextPhase(rm)
	if phase == nil {
		return NextPhaseOutput{Found: false}, nil
	}

	return NextPhaseOutput{
		Phase:      phase,
		ReadyItems: ReadyItems(phase),
		Found:      true,
	}, nil
}
