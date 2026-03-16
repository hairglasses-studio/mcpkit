package rdcycle

import (
	"testing"
)

func TestCostVelocityGovernorDefaults(t *testing.T) {
	g := NewCostVelocityGovernor(0, 0, 0)
	if g.WindowSize != 5 {
		t.Errorf("expected default window 5, got %d", g.WindowSize)
	}
	if g.AlarmPerCycle != 5.0 {
		t.Errorf("expected default alarm 5.0, got %f", g.AlarmPerCycle)
	}
	if g.UnproductiveCap != 3 {
		t.Errorf("expected default cap 3, got %d", g.UnproductiveCap)
	}
}

func TestCostVelocityGovernorNoHistory(t *testing.T) {
	g := NewCostVelocityGovernor(5, 5.0, 3)
	if g.Check() != nil {
		t.Error("expected nil alarm with no history")
	}
	if g.ShouldHalt() {
		t.Error("should not halt with no history")
	}
}

func TestCostVelocityGovernorUnderBudget(t *testing.T) {
	g := NewCostVelocityGovernor(5, 5.0, 3)
	g.Record(CycleRecord{CycleNum: 1, Cost: 2.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 2, Cost: 3.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 3, Cost: 1.0, Productive: true})

	if g.Check() != nil {
		t.Error("expected no alarm when under budget")
	}
	if g.ShouldHalt() {
		t.Error("should not halt when under budget and productive")
	}
}

func TestCostVelocityGovernorCostAlarm(t *testing.T) {
	g := NewCostVelocityGovernor(3, 5.0, 3)
	g.Record(CycleRecord{CycleNum: 1, Cost: 6.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 2, Cost: 7.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 3, Cost: 8.0, Productive: true})

	alarm := g.Check()
	if alarm == nil {
		t.Fatal("expected alarm when avg cost exceeds threshold")
	}
	if alarm.AvgCost <= 5.0 {
		t.Errorf("expected avg cost > 5.0, got %f", alarm.AvgCost)
	}
	if !g.ShouldHalt() {
		t.Error("should halt when cost velocity too high")
	}
}

func TestCostVelocityGovernorUnproductiveStreak(t *testing.T) {
	g := NewCostVelocityGovernor(5, 100.0, 3) // high alarm so only streak matters
	g.Record(CycleRecord{CycleNum: 1, Cost: 1.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 2, Cost: 1.0, Productive: false})
	g.Record(CycleRecord{CycleNum: 3, Cost: 1.0, Productive: false})

	if g.ShouldHalt() {
		t.Error("should not halt with only 2 unproductive cycles (cap=3)")
	}

	g.Record(CycleRecord{CycleNum: 4, Cost: 1.0, Productive: false})
	if !g.ShouldHalt() {
		t.Error("should halt after 3 consecutive unproductive cycles")
	}
}

func TestCostVelocityGovernorStreakResets(t *testing.T) {
	g := NewCostVelocityGovernor(5, 100.0, 3)
	g.Record(CycleRecord{CycleNum: 1, Cost: 1.0, Productive: false})
	g.Record(CycleRecord{CycleNum: 2, Cost: 1.0, Productive: false})
	g.Record(CycleRecord{CycleNum: 3, Cost: 1.0, Productive: true}) // reset
	g.Record(CycleRecord{CycleNum: 4, Cost: 1.0, Productive: false})
	g.Record(CycleRecord{CycleNum: 5, Cost: 1.0, Productive: false})

	if g.ShouldHalt() {
		t.Error("should not halt: streak reset by productive cycle")
	}
}

func TestCostVelocityGovernorRollingWindow(t *testing.T) {
	g := NewCostVelocityGovernor(3, 5.0, 10)

	// Old expensive cycles.
	g.Record(CycleRecord{CycleNum: 1, Cost: 20.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 2, Cost: 20.0, Productive: true})

	// Recent cheap cycles push the window average down.
	g.Record(CycleRecord{CycleNum: 3, Cost: 1.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 4, Cost: 1.0, Productive: true})
	g.Record(CycleRecord{CycleNum: 5, Cost: 1.0, Productive: true})

	// Window of last 3: avg = 1.0
	if g.Check() != nil {
		t.Error("rolling window should use only recent cycles")
	}
}

func TestCostVelocityGovernorTotalCost(t *testing.T) {
	g := NewCostVelocityGovernor(5, 100.0, 10)
	g.Record(CycleRecord{CycleNum: 1, Cost: 2.5})
	g.Record(CycleRecord{CycleNum: 2, Cost: 3.5})

	if got := g.TotalCost(); got != 6.0 {
		t.Errorf("expected total cost 6.0, got %f", got)
	}
}

func TestCostVelocityGovernorCycleCount(t *testing.T) {
	g := NewCostVelocityGovernor(5, 100.0, 10)
	g.Record(CycleRecord{CycleNum: 1, Cost: 1.0})
	g.Record(CycleRecord{CycleNum: 2, Cost: 1.0})

	if got := g.CycleCount(); got != 2 {
		t.Errorf("expected cycle count 2, got %d", got)
	}
}

func TestCostVelocityGovernorConcurrent(t *testing.T) {
	g := NewCostVelocityGovernor(5, 100.0, 10)
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 20; j++ {
				g.Record(CycleRecord{CycleNum: n*20 + j, Cost: 1.0, Productive: true})
				g.Check()
				g.ShouldHalt()
				g.TotalCost()
				g.CycleCount()
			}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
