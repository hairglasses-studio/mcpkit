package rdcycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Module implements registry.ToolModule for R&D cycle orchestration tools.
type Module struct {
	config       CycleConfig
	store        ArtifactStore
	orchMu       sync.Mutex
	orchState    *orchestratorState
	ralphStarter func(ctx context.Context, specPath string) error
	costReader   func() float64
}

// ModuleOption configures optional Module settings.
type ModuleOption func(*Module)

// WithArtifactStore sets the artifact store for the module.
// If not provided, an InMemoryArtifactStore is used.
func WithArtifactStore(store ArtifactStore) ModuleOption {
	return func(m *Module) {
		m.store = store
	}
}

// NewModule creates a new rdcycle Module with the given configuration.
// It uses an InMemoryArtifactStore by default. Use WithArtifactStore to override.
func NewModule(cfg CycleConfig, opts ...ModuleOption) *Module {
	m := &Module{
		config: cfg,
		store:  NewInMemoryArtifactStore(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Store returns the module's artifact store.
func (m *Module) Store() ArtifactStore {
	return m.store
}

// Name returns the module name.
func (m *Module) Name() string { return "rdcycle" }

// Description returns the module description.
func (m *Module) Description() string {
	return "R&D cycle orchestration tools: ecosystem scan, roadmap planning, build verification, and artifact management"
}

// SetRalphStarter sets the function used by the perpetual orchestrator to run ralph loops.
func (m *Module) SetRalphStarter(starter func(ctx context.Context, specPath string) error) {
	m.ralphStarter = starter
}

// SetCostReader sets the function used to read cumulative dollar cost for the governor.
func (m *Module) SetCostReader(reader func() float64) {
	m.costReader = reader
}

// Tools returns all rdcycle tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		m.scanTool(),
		m.planTool(),
		m.verifyTool(),
		m.artifactsTool(),
		m.commitTool(),
		m.reportTool(),
		m.scheduleTool(),
		m.notesTool(),
		m.improveTool(),
		m.perpetualStartTool(),
		m.perpetualStopTool(),
		m.perpetualStatusTool(),
	}
}

// ArtifactsInput is the input for the rdcycle_artifacts tool.
type ArtifactsInput struct {
	Type string `json:"type,omitempty" jsonschema:"description=Filter by artifact type: scan/plan/verify/code/test (default: all)"`
}

// ArtifactsOutput is the output of the rdcycle_artifacts tool.
type ArtifactsOutput struct {
	Artifacts []Artifact `json:"artifacts"`
	Count     int        `json:"count"`
}

func (m *Module) artifactsTool() registry.ToolDefinition {
	desc := "List artifacts stored during the current R&D cycle session. " +
		"Use type to filter by artifact kind: scan, plan, verify, code, or test. " +
		"Omit type to return all artifacts. " +
		"Artifacts are stored in-memory and reset when the server restarts."

	td := handler.TypedHandler[ArtifactsInput, ArtifactsOutput](
		"rdcycle_artifacts",
		desc,
		m.handleArtifacts,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.Complexity = registry.ComplexitySimple
	return td
}

func (m *Module) handleArtifacts(ctx context.Context, input ArtifactsInput) (ArtifactsOutput, error) {
	artifacts := m.store.List(input.Type)
	if artifacts == nil {
		artifacts = []Artifact{}
	}
	return ArtifactsOutput{
		Artifacts: artifacts,
		Count:     len(artifacts),
	}, nil
}

// Config returns the module's cycle configuration.
func (m *Module) Config() CycleConfig {
	return m.config
}

// HandleScan delegates to the scan handler.
func (m *Module) HandleScan(ctx context.Context, input ScanInput) (ScanOutput, error) {
	return m.handleScan(ctx, input)
}

// HandlePlan delegates to the plan handler.
func (m *Module) HandlePlan(ctx context.Context, input PlanInput) (PlanOutput, error) {
	return m.handlePlan(ctx, input)
}

// HandleNotes delegates to the notes handler.
func (m *Module) HandleNotes(ctx context.Context, input NotesInput) (NotesOutput, error) {
	return m.handleNotes(ctx, input)
}

// HandleImprove delegates to the improve handler.
func (m *Module) HandleImprove(ctx context.Context, input ImproveInput) (ImproveOutput, error) {
	return m.handleImprove(ctx, input)
}

// artifactID generates a unique artifact ID for the given type.
func artifactID(artifactType string) string {
	return fmt.Sprintf("%s-%d", artifactType, time.Now().UnixNano())
}
