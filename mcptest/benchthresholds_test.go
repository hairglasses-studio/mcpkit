package mcptest

import "testing"

func TestBenchmarkDeltaRegressedBySeparateThresholds(t *testing.T) {
	t.Parallel()

	thresholds := BenchmarkThresholds{
		NsPerOpPct:     10,
		BytesPerOpPct:  20,
		AllocsPerOpPct: 5,
	}

	cases := []struct {
		name  string
		delta BenchmarkDelta
		want  bool
	}{
		{
			name:  "latency",
			delta: BenchmarkDelta{NsPerOpPct: 10.1},
			want:  true,
		},
		{
			name:  "bytes",
			delta: BenchmarkDelta{BytesPerOpPct: 20.1},
			want:  true,
		},
		{
			name:  "allocs",
			delta: BenchmarkDelta{AllocsPerOpPct: 5.1},
			want:  true,
		},
		{
			name:  "at threshold",
			delta: BenchmarkDelta{NsPerOpPct: 10, BytesPerOpPct: 20, AllocsPerOpPct: 5},
			want:  false,
		},
		{
			name:  "improvement",
			delta: BenchmarkDelta{NsPerOpPct: -25, BytesPerOpPct: -10, AllocsPerOpPct: -1},
			want:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.delta.RegressedBy(thresholds); got != tc.want {
				t.Fatalf("RegressedBy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBenchmarkRegressions(t *testing.T) {
	t.Parallel()

	deltas := []BenchmarkDelta{
		{Name: "BenchmarkFast", NsPerOpPct: -5},
		{Name: "BenchmarkSlow", NsPerOpPct: 16},
		{Name: "BenchmarkAlloc", AllocsPerOpPct: 12},
	}
	regressions := BenchmarkRegressions(deltas, DefaultBenchmarkThresholds())
	if len(regressions) != 2 {
		t.Fatalf("regressions len = %d, want 2", len(regressions))
	}
	if regressions[0].Name != "BenchmarkSlow" || regressions[1].Name != "BenchmarkAlloc" {
		t.Fatalf("unexpected regressions: %+v", regressions)
	}
}
