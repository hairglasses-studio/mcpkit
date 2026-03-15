//go:build !official_sdk

package ralph

import (
	"context"
	"fmt"
	"sync"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// Module is a ToolModule that exposes ralph_start, ralph_stop, and ralph_status tools.
type Module struct {
	mu       sync.Mutex
	loop     *Loop
	registry *registry.ToolRegistry
	sampler  sampling.SamplingClient
}

// NewModule creates a ralph Module. The registry and sampler are used when starting loops.
func NewModule(reg *registry.ToolRegistry, sampler sampling.SamplingClient) *Module {
	return &Module{
		registry: reg,
		sampler:  sampler,
	}
}

func (m *Module) Name() string        { return "ralph" }
func (m *Module) Description() string { return "Autonomous loop runner for iterative task execution" }

type StartInput struct {
	SpecFile      string `json:"spec_file" jsonschema:"required,description=Path to the task specification JSON file"`
	MaxIterations int    `json:"max_iterations,omitempty" jsonschema:"description=Maximum loop iterations (default 100)"`
}

type StartOutput struct {
	Status   Status `json:"status"`
	SpecFile string `json:"spec_file"`
	Message  string `json:"message"`
}

type StopInput struct{}

type StopOutput struct {
	Status  Status `json:"status"`
	Message string `json:"message"`
}

type StatusInput struct{}

type StatusOutput struct {
	Status       Status   `json:"status"`
	Iteration    int      `json:"iteration"`
	CompletedIDs []string `json:"completed_ids"`
	SpecFile     string   `json:"spec_file"`
}

func (m *Module) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[StartInput, StartOutput](
			"ralph_start",
			"Start an autonomous loop that iteratively executes tasks from a spec file. The loop reads the spec, calls tools via LLM decisions, and tracks progress to disk. Use this to kick off automated multi-step workflows.",
			func(ctx context.Context, input StartInput) (StartOutput, error) {
				m.mu.Lock()
				defer m.mu.Unlock()

				if m.loop != nil {
					status := m.loop.Status()
					if status.Status == StatusRunning {
						return StartOutput{
							Status:   StatusRunning,
							SpecFile: status.SpecFile,
							Message:  "loop is already running",
						}, nil
					}
				}

				config := Config{
					SpecFile:      input.SpecFile,
					MaxIterations: input.MaxIterations,
					ToolRegistry:  m.registry,
					Sampler:       m.sampler,
				}

				loop, err := NewLoop(config)
				if err != nil {
					return StartOutput{}, fmt.Errorf("failed to create loop: %w", err)
				}
				m.loop = loop

				// Run in background goroutine
				go loop.Run(context.Background())

				return StartOutput{
					Status:   StatusRunning,
					SpecFile: input.SpecFile,
					Message:  "loop started",
				}, nil
			},
		),
		handler.TypedHandler[StopInput, StopOutput](
			"ralph_stop",
			"Stop the currently running autonomous loop. The loop will finish its current iteration and then stop gracefully.",
			func(ctx context.Context, input StopInput) (StopOutput, error) {
				m.mu.Lock()
				defer m.mu.Unlock()

				if m.loop == nil {
					return StopOutput{
						Status:  StatusIdle,
						Message: "no loop is running",
					}, nil
				}

				m.loop.Stop()
				status := m.loop.Status()
				return StopOutput{
					Status:  status.Status,
					Message: "stop signal sent",
				}, nil
			},
		),
		handler.TypedHandler[StatusInput, StatusOutput](
			"ralph_status",
			"Get the current status of the autonomous loop, including iteration count, completed task IDs, and overall status.",
			func(ctx context.Context, input StatusInput) (StatusOutput, error) {
				m.mu.Lock()
				defer m.mu.Unlock()

				if m.loop == nil {
					return StatusOutput{Status: StatusIdle}, nil
				}

				p := m.loop.Status()
				return StatusOutput{
					Status:       p.Status,
					Iteration:    p.Iteration,
					CompletedIDs: p.CompletedIDs,
					SpecFile:     p.SpecFile,
				}, nil
			},
		),
	}
}
