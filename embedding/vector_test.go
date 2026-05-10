package embedding

import (
	"errors"
	"math"
	"testing"
)

func TestSparseCosine(t *testing.T) {
	a := SparseVector{"roadmap": 1, "cache": 1}
	b := SparseVector{"roadmap": 1, "cache": 1}
	c := SparseVector{"evidence": 1}

	if got := Cosine(a, b); math.Abs(got-1) > 1e-9 {
		t.Fatalf("Cosine identical = %v, want 1", got)
	}
	if got := Cosine(a, c); got != 0 {
		t.Fatalf("Cosine disjoint = %v, want 0", got)
	}
}

func TestTopKSortsAndCaps(t *testing.T) {
	got := TopK([]Score{
		{ID: "b", Score: 0.2},
		{ID: "c", Score: 0.9},
		{ID: "a", Score: 0.9},
	}, 2)
	want := []Score{
		{ID: "a", Score: 0.9},
		{ID: "c", Score: 0.9},
	}
	if len(got) != len(want) {
		t.Fatalf("TopK length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TopK()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDenseCosineDimensionMismatch(t *testing.T) {
	_, err := DenseCosine(DenseVector{1, 2}, DenseVector{1})
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("DenseCosine err = %v, want ErrDimensionMismatch", err)
	}
}
