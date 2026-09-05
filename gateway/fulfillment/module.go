package fulfillment

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// FulfillmentModule exposes tools for the self-fulfillment matrix under MCP.
type FulfillmentModule struct {
	matrixMgr *MatrixManager
	engine    *PipelineEngine
}

// NewFulfillmentModule initializes a new fulfillment MCP module.
func NewFulfillmentModule(deployDir string) *FulfillmentModule {
	return &FulfillmentModule{
		matrixMgr: NewMatrixManager(deployDir),
		engine:    NewPipelineEngine(deployDir),
	}
}

func (m *FulfillmentModule) Name() string {
	return "fulfillment"
}

func (m *FulfillmentModule) Description() string {
	return "Unified Go MCP Gateway for End-to-End Self-Fulfillment Matrix Automation"
}

// Tools registers all typed handlers for the self-fulfillment matrix.
func (m *FulfillmentModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[MatrixStatusInput, MatrixStatusOutput](
			"matrix_status",
			"Query status, metrics, counts, and active leases across the self-fulfillment matrix.",
			func(_ context.Context, input MatrixStatusInput) (MatrixStatusOutput, error) {
				return m.matrixMgr.GetStatus(input.Verbose)
			},
		),

		handler.TypedHandler[ScanGapsInput, ScanGapsOutput](
			"matrix_scan_gaps",
			"Scan and list unfulfilled sequence gaps and missing numbers across media series.",
			func(_ context.Context, input ScanGapsInput) (ScanGapsOutput, error) {
				gaps, err := m.matrixMgr.ScanGaps(input.SeriesName, input.Limit)
				if err != nil {
					return ScanGapsOutput{}, err
				}
				highPrio := 0
				for _, g := range gaps {
					if g.Priority >= 8 {
						highPrio++
					}
				}
				return ScanGapsOutput{
					TotalGapsFound:    len(gaps),
					HighPriorityCount: highPrio,
					Gaps:              gaps,
				}, nil
			},
		),

		handler.TypedHandler[ClaimBatchInput, ClaimBatchOutput](
			"matrix_claim_batch",
			"Acquire an exclusive lease on a sequence gap batch or dedupe range to prevent multi-agent collision.",
			func(_ context.Context, input ClaimBatchInput) (ClaimBatchOutput, error) {
				return m.matrixMgr.ClaimBatch(input.BatchKey, input.Scope, input.OwnerSession, input.LeaseDurationSec)
			},
		),

		handler.TypedHandler[ExecuteStepInput, ExecuteStepOutput](
			"matrix_execute_step",
			"Execute a designated stage in the self-fulfillment pipeline with safety timeout.",
			func(ctx context.Context, input ExecuteStepInput) (ExecuteStepOutput, error) {
				res := m.engine.ExecuteStep(ctx, input.StepName, input.DryRun, input.TimeoutSec)
				return res, nil
			},
		),

		handler.TypedHandler[CheckInvariantsInput, CheckInvariantsOutput](
			"matrix_check_invariants",
			"Verify system safety invariants (NVMe storage floor, GPU VRAM ceiling, zero desktop toasts, StashApp GraphQL status).",
			func(ctx context.Context, input CheckInvariantsInput) (CheckInvariantsOutput, error) {
				report, details := CheckInvariants(ctx, input.MinNVMeFreeGB, input.MaxVRAMUsedMB)
				return CheckInvariantsOutput{Report: report, Details: details}, nil
			},
		),

		handler.TypedHandler[SyncMailboxInput, SyncMailboxOutput](
			"matrix_sync_mailbox",
			"Broadcast completed deliverables and in-flight tasks to the central fleet mailbox hubs.",
			func(_ context.Context, input SyncMailboxInput) (SyncMailboxOutput, error) {
				return SyncMailbox(input.SessionID, input.TeamID, input.Subject, input.Deliverables, input.InFlightTasks)
			},
		),

		handler.TypedHandler[RunE2EInput, RunE2EOutput](
			"matrix_run_e2e",
			"Orchestrate complete end-to-end self-fulfillment loop: check invariants -> scan gaps -> execute steps -> sync mailbox.",
			func(ctx context.Context, input RunE2EInput) (RunE2EOutput, error) {
				out := m.engine.RunE2E(ctx, m.matrixMgr, input.DryRun, input.MaxSteps, input.SessionID)
				return out, nil
			},
		),
	}
}
