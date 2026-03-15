package rdcycle

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Module implements registry.ToolModule for R&D cycle orchestration tools.
type Module struct {
	config CycleConfig
	store  ArtifactStore
}

// NewModule creates a new rdcycle Module with the given configuration.
// It uses an InMemoryArtifactStore by default.
func NewModule(cfg CycleConfig) *Module {
	return &Module{
		config: cfg,
		store:  NewInMemoryArtifactStore(),
	}
}

// Name returns the module name.
func (m *Module) Name() string { return "rdcycle" }

// Description returns the module description.
func (m *Module) Description() string {
	return "R&D cycle orchestration tools: ecosystem scan, roadmap planning, build verification, and artifact management"
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

// artifactID generates a unique artifact ID for the given type.
func artifactID(artifactType string) string {
	return fmt.Sprintf("%s-%d", artifactType, time.Now().UnixNano())
}
