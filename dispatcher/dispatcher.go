package dispatcher

import (
	"container/heap"
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// PriorityFunc determines job priority from tool name and definition.
type PriorityFunc func(name string, td registry.ToolDefinition) Priority

// GroupFunc determines concurrency group from tool name and definition.
type GroupFunc func(name string, td registry.ToolDefinition) string

// Config configures the dispatcher.
type Config struct {
	// Workers is the number of worker goroutines. Default: runtime.GOMAXPROCS(0).
	Workers int
	// QueueSize is the maximum number of pending jobs. Default: 1000.
	QueueSize int
	// GroupLimits maps concurrency group names to max concurrent executions.
	GroupLimits map[string]int
	// DefaultPriority for jobs when PriorityFunc is nil. Default: PriorityNormal.
	DefaultPriority Priority
	// PriorityFunc determines priority per tool. Overrides DefaultPriority.
	PriorityFunc PriorityFunc
	// GroupFunc determines group per tool. Overrides ToolDefinition fields.
	GroupFunc GroupFunc
	// ShutdownTimeout is the max time to wait for in-flight jobs during shutdown.
	// Default: 30s.
	ShutdownTimeout time.Duration
}

// Stats is a snapshot of dispatcher state.
type Stats struct {
	QueueDepth    int            `json:"queue_depth"`
	ActiveWorkers int            `json:"active_workers"`
	TotalWorkers  int            `json:"total_workers"`
	Submitted     uint64         `json:"submitted"`
	Completed     uint64         `json:"completed"`
	Failed        uint64         `json:"failed"`
	GroupActive   map[string]int `json:"group_active"`
}

// Dispatcher schedules tool-handler Jobs across a fixed pool of workers,
// respecting per-job priority and per-group concurrency limits.
type Dispatcher struct {
	cfg  Config
	mu   sync.Mutex
	cond *sync.Cond

	queue  priorityQueue
	groups *groupManager

	seq    uint64 // monotonic FIFO counter, protected by mu
	closed bool

	submitted     uint64 // atomic
	completed     uint64 // atomic
	failed        uint64 // atomic
	activeWorkers int64  // atomic

	wg sync.WaitGroup
}

// New creates a Dispatcher with the given configuration, applies defaults,
// and starts the worker goroutines.
func New(config Config) *Dispatcher {
	if config.Workers <= 0 {
		config.Workers = runtime.GOMAXPROCS(0)
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 30 * time.Second
	}
	// DefaultPriority zero value is PriorityLow; default to PriorityNormal.
	// We only override when it's the zero value AND no PriorityFunc is set,
	// but the spec says "Default: PriorityNormal" so set it unconditionally
	// when it equals the zero value (PriorityLow == 0).
	// Actually the spec says the field default is PriorityNormal, so we set
	// it when the caller left it as the zero value.
	if config.DefaultPriority == 0 && config.PriorityFunc == nil {
		config.DefaultPriority = PriorityNormal
	}

	d := &Dispatcher{
		cfg:    config,
		queue:  make(priorityQueue, 0, config.QueueSize),
		groups: newGroupManager(config.GroupLimits),
	}
	d.cond = sync.NewCond(&d.mu)

	heap.Init(&d.queue)

	d.wg.Add(config.Workers)
	for i := 0; i < config.Workers; i++ {
		go d.worker()
	}

	return d
}

// Submit enqueues a Job and waits for its result, respecting ctx cancellation.
// It returns the handler's result and error. Dispatcher-level errors (queue
// full, shut down) are returned as an error CallToolResult with a nil error.
func (d *Dispatcher) Submit(ctx context.Context, job *Job) (*registry.CallToolResult, error) {
	// Apply priority and group from config funcs if set.
	if d.cfg.PriorityFunc != nil {
		job.Priority = d.cfg.PriorityFunc(job.Name, job.TD)
	} else if job.Priority == 0 {
		job.Priority = d.cfg.DefaultPriority
	}
	if d.cfg.GroupFunc != nil {
		job.Group = d.cfg.GroupFunc(job.Name, job.TD)
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return registry.MakeErrorResult("dispatcher is shut down"), nil
	}
	if d.queue.Len() >= d.cfg.QueueSize {
		d.mu.Unlock()
		return registry.MakeErrorResult("dispatcher queue is full"), nil
	}

	job.seq = atomic.AddUint64(&d.seq, 1)
	job.result = make(chan jobResult, 1)
	heap.Push(&d.queue, job)
	atomic.AddUint64(&d.submitted, 1)
	d.cond.Broadcast()
	d.mu.Unlock()

	select {
	case res := <-job.result:
		return res.Result, res.Err
	case <-ctx.Done():
		return registry.MakeErrorResult(ctx.Err().Error()), nil
	}
}

// Shutdown signals the dispatcher to stop accepting new jobs and waits for
// in-flight jobs to complete. ctx controls the maximum wait duration.
func (d *Dispatcher) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	d.closed = true
	d.cond.Broadcast()
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns a point-in-time snapshot of dispatcher metrics.
func (d *Dispatcher) Stats() Stats {
	d.mu.Lock()
	qDepth := d.queue.Len()
	groupActive := d.groups.snapshot()
	d.mu.Unlock()

	return Stats{
		QueueDepth:    qDepth,
		ActiveWorkers: int(atomic.LoadInt64(&d.activeWorkers)),
		TotalWorkers:  d.cfg.Workers,
		Submitted:     atomic.LoadUint64(&d.submitted),
		Completed:     atomic.LoadUint64(&d.completed),
		Failed:        atomic.LoadUint64(&d.failed),
		GroupActive:   groupActive,
	}
}

// worker is the goroutine that dequeues and executes Jobs.
func (d *Dispatcher) worker() {
	defer d.wg.Done()

	for {
		d.mu.Lock()

		var job *Job
		for {
			job = d.dequeue()
			if job != nil {
				break
			}
			if d.closed && d.queue.Len() == 0 {
				d.mu.Unlock()
				return
			}
			d.cond.Wait()
		}

		d.groups.acquire(job.Group)
		d.mu.Unlock()

		atomic.AddInt64(&d.activeWorkers, 1)
		result, err := job.Handler(job.Ctx, job.Request)
		atomic.AddInt64(&d.activeWorkers, -1)

		atomic.AddUint64(&d.completed, 1)
		if err != nil || registry.IsResultError(result) {
			atomic.AddUint64(&d.failed, 1)
		}

		// Buffered channel — never blocks.
		job.result <- jobResult{Result: result, Err: err}

		d.mu.Lock()
		d.groups.release(job.Group)
		d.cond.Broadcast()
		d.mu.Unlock()
	}
}

// dequeue scans the priority queue for the first job whose group is eligible
// and removes it. Returns nil when no eligible job exists.
// Must be called with d.mu held.
func (d *Dispatcher) dequeue() *Job {
	for i, job := range d.queue {
		if d.groups.canAcquire(job.Group) {
			heap.Remove(&d.queue, i)
			return job
		}
	}
	return nil
}
