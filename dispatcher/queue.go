package dispatcher

// priorityQueue implements container/heap.Interface for *Job.
// Higher priority values execute first; within the same priority,
// lower seq values execute first (FIFO).
type priorityQueue []*Job

func (pq priorityQueue) Len() int { return len(pq) }

// Less returns true when item i should be dequeued before item j.
// heap.Pop removes the element for which Less(i, j) is never true for any
// other j — i.e., the "minimum" element — so we invert our desired ordering:
// higher priority is "less", and for equal priority, lower seq is "less".
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].seq < pq[j].seq
}

func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

// Push appends a new *Job to the queue (called by heap.Push).
func (pq *priorityQueue) Push(x any) {
	*pq = append(*pq, x.(*Job))
}

// Pop removes and returns the last element (called by heap.Pop after heap
// has swapped the top element to the end).
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	job := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[:n-1]
	return job
}
