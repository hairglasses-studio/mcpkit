package mcptest

// BenchmarkThresholds configures per-metric regression budgets for benchmark
// comparisons. A metric regresses when the percentage delta is greater than
// its threshold.
type BenchmarkThresholds struct {
	NsPerOpPct     float64
	BytesPerOpPct  float64
	AllocsPerOpPct float64
}

// DefaultBenchmarkThresholds returns conservative defaults for noisy CI hosts.
func DefaultBenchmarkThresholds() BenchmarkThresholds {
	return BenchmarkThresholds{
		NsPerOpPct:     15,
		BytesPerOpPct:  20,
		AllocsPerOpPct: 10,
	}
}

// RegressedBy returns true when any metric exceeds its configured threshold.
func (d BenchmarkDelta) RegressedBy(thresholds BenchmarkThresholds) bool {
	return d.NsPerOpPct > thresholds.NsPerOpPct ||
		d.BytesPerOpPct > thresholds.BytesPerOpPct ||
		d.AllocsPerOpPct > thresholds.AllocsPerOpPct
}

// BenchmarkRegressions filters benchmark deltas to only those that exceed at
// least one threshold.
func BenchmarkRegressions(deltas []BenchmarkDelta, thresholds BenchmarkThresholds) []BenchmarkDelta {
	regressions := make([]BenchmarkDelta, 0)
	for _, delta := range deltas {
		if delta.RegressedBy(thresholds) {
			regressions = append(regressions, delta)
		}
	}
	return regressions
}
