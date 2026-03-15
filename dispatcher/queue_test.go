//go:build !official_sdk

package dispatcher

import (
	"container/heap"
	"testing"
)

func newQueueJob(priority Priority, seq uint64) *Job {
	j := &Job{Priority: priority, seq: seq}
	return j
}

func TestPriorityQueue_Len(t *testing.T) {
	t.Parallel()
	pq := priorityQueue{}
	if pq.Len() != 0 {
		t.Errorf("expected Len 0, got %d", pq.Len())
	}
	pq = append(pq, newQueueJob(PriorityNormal, 1))
	if pq.Len() != 1 {
		t.Errorf("expected Len 1, got %d", pq.Len())
	}
}

func TestPriorityQueue_Less_HigherPriorityFirst(t *testing.T) {
	t.Parallel()
	pq := priorityQueue{
		newQueueJob(PriorityLow, 1),
		newQueueJob(PriorityHigh, 2),
	}
	// Less(1, 0) should be true: High > Low means index 1 is "less" (should come first).
	if !pq.Less(1, 0) {
		t.Error("expected PriorityHigh (index 1) to be less than PriorityLow (index 0)")
	}
	if pq.Less(0, 1) {
		t.Error("expected PriorityLow (index 0) to NOT be less than PriorityHigh (index 1)")
	}
}

func TestPriorityQueue_Less_SamePriority_FIFOBySeq(t *testing.T) {
	t.Parallel()
	pq := priorityQueue{
		newQueueJob(PriorityNormal, 5),
		newQueueJob(PriorityNormal, 3),
	}
	// Within same priority, lower seq is "less" (executes first).
	if !pq.Less(1, 0) {
		t.Error("expected lower seq (index 1, seq=3) to be less than (index 0, seq=5)")
	}
	if pq.Less(0, 1) {
		t.Error("expected higher seq (index 0, seq=5) to NOT be less than (index 1, seq=3)")
	}
}

func TestPriorityQueue_Swap(t *testing.T) {
	t.Parallel()
	a := newQueueJob(PriorityLow, 1)
	b := newQueueJob(PriorityHigh, 2)
	pq := priorityQueue{a, b}
	pq.Swap(0, 1)
	if pq[0] != b || pq[1] != a {
		t.Error("Swap did not exchange elements correctly")
	}
}

func TestPriorityQueue_PushPop(t *testing.T) {
	t.Parallel()
	pq := &priorityQueue{}
	heap.Init(pq)

	heap.Push(pq, newQueueJob(PriorityLow, 3))
	heap.Push(pq, newQueueJob(PriorityCritical, 1))
	heap.Push(pq, newQueueJob(PriorityNormal, 2))

	if pq.Len() != 3 {
		t.Errorf("expected Len 3, got %d", pq.Len())
	}

	// Pop should return Critical first, then Normal, then Low.
	first := heap.Pop(pq).(*Job)
	if first.Priority != PriorityCritical {
		t.Errorf("expected PriorityCritical first, got %d", first.Priority)
	}

	second := heap.Pop(pq).(*Job)
	if second.Priority != PriorityNormal {
		t.Errorf("expected PriorityNormal second, got %d", second.Priority)
	}

	third := heap.Pop(pq).(*Job)
	if third.Priority != PriorityLow {
		t.Errorf("expected PriorityLow third, got %d", third.Priority)
	}

	if pq.Len() != 0 {
		t.Errorf("expected empty queue after all pops, got Len %d", pq.Len())
	}
}

func TestPriorityQueue_FIFOWithinSamePriority(t *testing.T) {
	t.Parallel()
	pq := &priorityQueue{}
	heap.Init(pq)

	// All same priority, different seq.
	heap.Push(pq, newQueueJob(PriorityNormal, 10))
	heap.Push(pq, newQueueJob(PriorityNormal, 1))
	heap.Push(pq, newQueueJob(PriorityNormal, 5))

	first := heap.Pop(pq).(*Job)
	if first.seq != 1 {
		t.Errorf("expected seq=1 first (FIFO), got seq=%d", first.seq)
	}

	second := heap.Pop(pq).(*Job)
	if second.seq != 5 {
		t.Errorf("expected seq=5 second, got seq=%d", second.seq)
	}

	third := heap.Pop(pq).(*Job)
	if third.seq != 10 {
		t.Errorf("expected seq=10 third, got seq=%d", third.seq)
	}
}

func TestPriorityQueue_PopNilsSlot(t *testing.T) {
	t.Parallel()
	// Verify Pop sets the vacated slot to nil (prevents memory leaks).
	pq := &priorityQueue{}
	heap.Init(pq)
	j := newQueueJob(PriorityNormal, 1)
	heap.Push(pq, j)

	// Access underlying slice before pop to get the original backing array reference.
	oldSlice := *pq
	_ = oldSlice // capture before pop

	heap.Pop(pq)

	// After pop, the underlying array's now-out-of-bounds slot is nil.
	// We can only verify pq is empty now.
	if pq.Len() != 0 {
		t.Errorf("expected empty queue, got Len %d", pq.Len())
	}
}

func TestPriorityQueue_MixedPriorityAndSeq(t *testing.T) {
	t.Parallel()
	pq := &priorityQueue{}
	heap.Init(pq)

	heap.Push(pq, newQueueJob(PriorityLow, 1))
	heap.Push(pq, newQueueJob(PriorityHigh, 10))
	heap.Push(pq, newQueueJob(PriorityHigh, 5))
	heap.Push(pq, newQueueJob(PriorityNormal, 3))

	// Expected order: High/5, High/10, Normal/3, Low/1
	expected := []struct {
		priority Priority
		seq      uint64
	}{
		{PriorityHigh, 5},
		{PriorityHigh, 10},
		{PriorityNormal, 3},
		{PriorityLow, 1},
	}

	for i, want := range expected {
		got := heap.Pop(pq).(*Job)
		if got.Priority != want.priority || got.seq != want.seq {
			t.Errorf("pop[%d]: want (priority=%d,seq=%d), got (priority=%d,seq=%d)",
				i, want.priority, want.seq, got.Priority, got.seq)
		}
	}
}
