package fulfillment

import (
	"context"
	"testing"
)

func TestCheckInvariants(t *testing.T) {
	ctx := context.Background()
	report, details := CheckInvariants(ctx, 50.0, 15000.0)

	if report.NVMeFreeGB <= 0 {
		t.Errorf("Expected positive NVMe free space, got %.2f", report.NVMeFreeGB)
	}
	if report.VRAMUsedMB <= 0 {
		t.Errorf("Expected positive VRAM reading, got %.2f", report.VRAMUsedMB)
	}
	if len(details) == 0 {
		t.Errorf("Expected details slice to be populated")
	}
}

func TestMatrixManagerStatus(t *testing.T) {
	mm := NewMatrixManager("")
	status, err := mm.GetStatus(false)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.ActiveExclusiveLeases) == 0 {
		t.Errorf("Expected seeded active exclusive leases")
	}
}

func TestMatrixManagerScanGaps(t *testing.T) {
	mm := NewMatrixManager("")
	gaps, err := mm.ScanGaps("DF", 5)
	if err != nil {
		t.Fatalf("ScanGaps failed: %v", err)
	}
	if len(gaps) == 0 {
		t.Fatalf("Expected gaps for series DF")
	}
	for _, g := range gaps {
		if g.Series != "DF" {
			t.Errorf("Expected series DF, got %s", g.Series)
		}
	}
}

func TestMatrixManagerRawAndStudioLeases(t *testing.T) {
	mm := NewMatrixManager("")

	// Scan RAW gaps
	rawGaps, err := mm.ScanGaps("RAW", 10)
	if err != nil {
		t.Fatalf("ScanGaps RAW failed: %v", err)
	}
	if len(rawGaps) != 2 {
		t.Fatalf("Expected 2 RAW gaps, got %d", len(rawGaps))
	}
	for _, g := range rawGaps {
		if g.Status != "LOCKED_matrix-fulfillment-specialist" {
			t.Errorf("Expected status LOCKED_matrix-fulfillment-specialist for RAW %s, got %s", g.MissingID, g.Status)
		}
	}

	// Scan STUDIO gaps
	studioGaps, err := mm.ScanGaps("STUDIO", 10)
	if err != nil {
		t.Fatalf("ScanGaps STUDIO failed: %v", err)
	}
	if len(studioGaps) != 1 {
		t.Fatalf("Expected 1 STUDIO gap, got %d", len(studioGaps))
	}
	if studioGaps[0].MissingID != "042" {
		t.Errorf("Expected STUDIO gap 042, got %s", studioGaps[0].MissingID)
	}
	if studioGaps[0].Status != "LOCKED_matrix-fulfillment-specialist" {
		t.Errorf("Expected status LOCKED_matrix-fulfillment-specialist for STUDIO 042, got %s", studioGaps[0].Status)
	}
}

func TestMatrixManagerClaimLease(t *testing.T) {
	mm := NewMatrixManager("")

	// 1. Initial claim
	res, err := mm.ClaimBatch("test_batch_1", "TEST #01-#10", "session_alpha", 60)
	if err != nil {
		t.Fatalf("ClaimBatch failed: %v", err)
	}
	if !res.Granted {
		t.Errorf("Expected lease to be granted")
	}

	// 2. Collision claim from another session
	res2, err := mm.ClaimBatch("test_batch_1", "TEST #01-#10", "session_beta", 60)
	if err != nil {
		t.Fatalf("ClaimBatch failed: %v", err)
	}
	if res2.Granted {
		t.Errorf("Expected lease collision to be rejected")
	}
	if res2.Owner != "session_alpha" {
		t.Errorf("Expected owner session_alpha, got %s", res2.Owner)
	}
}

func TestPipelineEngineExecuteStep(t *testing.T) {
	engine := NewPipelineEngine("")
	ctx := context.Background()

	res := engine.ExecuteStep(ctx, "phase_a", true, 10)
	if res.Status != "SUCCESS_DRY_RUN" {
		t.Errorf("Expected SUCCESS_DRY_RUN, got %s", res.Status)
	}
}

func TestPipelineEngineRunE2E(t *testing.T) {
	engine := NewPipelineEngine("")
	mm := NewMatrixManager("")
	ctx := context.Background()

	out := engine.RunE2E(ctx, mm, true, 2, "test_session_e2e")
	if !out.Success {
		t.Errorf("Expected E2E run to succeed, got summary: %s", out.Summary)
	}
	if len(out.StepsExecuted) == 0 {
		t.Errorf("Expected executed steps")
	}
}

func TestFulfillmentModuleRegistration(t *testing.T) {
	mod := NewFulfillmentModule("")
	if mod.Name() != "fulfillment" {
		t.Errorf("Expected module name fulfillment, got %s", mod.Name())
	}
	tools := mod.Tools()
	if len(tools) < 7 {
		t.Errorf("Expected at least 7 tools, got %d", len(tools))
	}
}
