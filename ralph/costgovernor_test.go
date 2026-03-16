package ralph

import (
	"testing"
)

func TestCostGovernor_OKByDefault(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{})
	v := cg.Check()
	if v.Action != "ok" {
		t.Errorf("expected ok, got %q", v.Action)
	}
}

func TestCostGovernor_HardBudgetHalt(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{HardBudgetTokens: 100})
	cg.RecordIteration(101, true)
	v := cg.Check()
	if v.Action != "halt" {
		t.Errorf("expected halt, got %q", v.Action)
	}
	if v.Warning == "" {
		t.Error("expected non-empty warning")
	}
}

func TestCostGovernor_HardBudgetNotTriggeredBelowLimit(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{HardBudgetTokens: 100})
	cg.RecordIteration(99, true)
	v := cg.Check()
	if v.Action != "ok" {
		t.Errorf("expected ok below budget, got %q", v.Action)
	}
}

func TestCostGovernor_UnproductiveStreakHalt(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{UnproductiveMax: 3})
	for i := 0; i < 3; i++ {
		cg.RecordIteration(10, false)
	}
	v := cg.Check()
	if v.Action != "halt" {
		t.Errorf("expected halt after unproductive streak, got %q", v.Action)
	}
}

func TestCostGovernor_StreakResetOnProgress(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{UnproductiveMax: 3})
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, true) // progress resets streak
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	v := cg.Check()
	if v.Action != "ok" {
		t.Errorf("expected ok after streak reset, got %q (streak=%d)", v.Action, cg.UnproductiveStreak())
	}
}

func TestCostGovernor_VelocityAlarmDowngrade(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{
		VelocityWindow:    4,
		VelocityAlarmRate: 0.5, // 50% threshold
		UnproductiveMax:   100,
	})
	// 3 out of 4 unproductive = 75% > 50%
	cg.RecordIteration(10, true)
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	v := cg.Check()
	if v.Action != "downgrade" {
		t.Errorf("expected downgrade, got %q", v.Action)
	}
	if v.Warning == "" {
		t.Error("expected non-empty warning")
	}
}

func TestCostGovernor_VelocityNotCheckedBelowWindow(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{
		VelocityWindow:    5,
		VelocityAlarmRate: 0.5,
		UnproductiveMax:   100,
	})
	// Only 3 iterations — below window of 5.
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	v := cg.Check()
	if v.Action != "ok" {
		t.Errorf("expected ok below velocity window, got %q", v.Action)
	}
}

func TestCostGovernor_HardBudgetPriorityOverStreak(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{
		HardBudgetTokens: 50,
		UnproductiveMax:  1,
	})
	cg.RecordIteration(100, false)
	v := cg.Check()
	// Both hard budget and streak trigger; hard budget should come first.
	if v.Action != "halt" {
		t.Errorf("expected halt, got %q", v.Action)
	}
	// Warning should mention budget.
	if len(v.Warning) == 0 {
		t.Error("expected non-empty warning")
	}
}

func TestCostGovernor_TotalTokens(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{})
	cg.RecordIteration(100, true)
	cg.RecordIteration(200, false)
	if cg.TotalTokens() != 300 {
		t.Errorf("expected 300, got %d", cg.TotalTokens())
	}
}

func TestCostGovernor_UnproductiveStreakAccessor(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{})
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	if cg.UnproductiveStreak() != 2 {
		t.Errorf("expected streak=2, got %d", cg.UnproductiveStreak())
	}
	cg.RecordIteration(10, true)
	if cg.UnproductiveStreak() != 0 {
		t.Errorf("expected streak=0 after progress, got %d", cg.UnproductiveStreak())
	}
}

func TestCostGovernor_DefaultZeroConfig(t *testing.T) {
	cfg := DefaultCostGovernorConfig()
	if cfg.HardBudgetTokens != 0 {
		t.Errorf("expected HardBudgetTokens=0, got %d", cfg.HardBudgetTokens)
	}
	if cfg.VelocityWindow != 5 {
		t.Errorf("expected VelocityWindow=5, got %d", cfg.VelocityWindow)
	}
	if cfg.VelocityAlarmRate != 0 {
		t.Errorf("expected VelocityAlarmRate=0, got %f", cfg.VelocityAlarmRate)
	}
	if cfg.UnproductiveMax != 3 {
		t.Errorf("expected UnproductiveMax=3, got %d", cfg.UnproductiveMax)
	}
}

func TestCostGovernor_ZeroVelocityRateDisables(t *testing.T) {
	cg := NewCostGovernor(CostGovernorConfig{
		VelocityWindow:    3,
		VelocityAlarmRate: 0, // disabled
		UnproductiveMax:   100,
	})
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	cg.RecordIteration(10, false)
	v := cg.Check()
	if v.Action != "ok" {
		t.Errorf("expected ok with velocity disabled, got %q", v.Action)
	}
}
