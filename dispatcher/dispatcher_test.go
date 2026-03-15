package dispatcher

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// makeHandler returns a ToolHandlerFunc that returns a text result.
func makeHandler(text string) registry.ToolHandlerFunc {
	return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult(text), nil
	}
}

// makeSlowHandler sleeps for d before returning.
func makeSlowHandler(d time.Duration, text string) registry.ToolHandlerFunc {
	return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		time.Sleep(d)
		return registry.MakeTextResult(text), nil
	}
}

func newJob(name string, handler registry.ToolHandlerFunc) *Job {
	return &Job{
		Name:    name,
		Ctx:     context.Background(),
		Handler: handler,
	}
}

func TestNew_defaults(t *testing.T) {
	d := New(Config{})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	s := d.Stats()
	if s.TotalWorkers <= 0 {
		t.Errorf("expected TotalWorkers > 0, got %d", s.TotalWorkers)
	}
	if d.cfg.QueueSize != 1000 {
		t.Errorf("expected QueueSize 1000, got %d", d.cfg.QueueSize)
	}
	if d.cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("expected ShutdownTimeout 30s, got %v", d.cfg.ShutdownTimeout)
	}
	if d.cfg.DefaultPriority != PriorityNormal {
		t.Errorf("expected DefaultPriority PriorityNormal, got %v", d.cfg.DefaultPriority)
	}
}

func TestSubmit_basic(t *testing.T) {
	d := New(Config{Workers: 2})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	result, err := d.Submit(context.Background(), newJob("echo", makeHandler("hello")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Errorf("expected non-error result, got error")
	}
}

func TestSubmit_afterShutdown(t *testing.T) {
	d := New(Config{Workers: 1})
	ctx := context.Background()
	_ = d.Shutdown(ctx)

	result, err := d.Submit(ctx, newJob("echo", makeHandler("hello")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result when shut down")
	}
}

func TestSubmit_queueFull(t *testing.T) {
	// Single slow worker, tiny queue — fill the queue then submit one more.
	d := New(Config{Workers: 1, QueueSize: 2})

	// Block the worker with a long-running job.
	blocker := make(chan struct{})
	blocking := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		<-blocker
		return registry.MakeTextResult("done"), nil
	}

	// Submit the blocker job first (worker picks it up immediately).
	// Then fill the queue.
	go d.Submit(context.Background(), newJob("block", blocking)) //nolint:errcheck

	// Give the worker time to pick up the blocker.
	time.Sleep(20 * time.Millisecond)

	// Fill the queue.
	go d.Submit(context.Background(), newJob("q1", makeHandler("a"))) //nolint:errcheck
	go d.Submit(context.Background(), newJob("q2", makeHandler("b"))) //nolint:errcheck

	time.Sleep(20 * time.Millisecond)

	// One more should hit the full queue.
	result, err := d.Submit(context.Background(), newJob("overflow", makeHandler("x")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result for full queue")
	}

	close(blocker) // unblock worker so shutdown can complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d.Shutdown(shutdownCtx) //nolint:errcheck
}

func TestSubmit_contextCancellation(t *testing.T) {
	// One slow worker, one job enqueued but context cancelled before result.
	d := New(Config{Workers: 1})

	blocker := make(chan struct{})
	go d.Submit(context.Background(), newJob("block", func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) { //nolint:errcheck
		<-blocker
		return registry.MakeTextResult("done"), nil
	}))

	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	result, err := d.Submit(ctx, newJob("cancelled", makeHandler("never")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result for cancelled context")
	}

	close(blocker)
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	d.Shutdown(shutdownCtx) //nolint:errcheck
}

func TestStats_counters(t *testing.T) {
	d := New(Config{Workers: 4})
	ctx := context.Background()

	const n = 10
	for i := 0; i < n; i++ {
		_, err := d.Submit(ctx, newJob("t", makeHandler("ok")))
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}

	s := d.Stats()
	if s.Submitted != n {
		t.Errorf("Submitted: want %d, got %d", n, s.Submitted)
	}
	if s.Completed != n {
		t.Errorf("Completed: want %d, got %d", n, s.Completed)
	}
	if s.Failed != 0 {
		t.Errorf("Failed: want 0, got %d", s.Failed)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestPriorityOrdering(t *testing.T) {
	// Single worker, slow blocker first to fill the queue,
	// then enqueue low and high priority — high must run first.
	d := New(Config{Workers: 1, QueueSize: 100})

	order := make([]string, 0, 3)
	var mu sync.Mutex
	record := func(name string) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return registry.MakeTextResult(name), nil
		}
	}

	blocker := make(chan struct{})
	go d.Submit(context.Background(), newJob("block", func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) { //nolint:errcheck
		<-blocker
		return registry.MakeTextResult("block"), nil
	}))

	time.Sleep(10 * time.Millisecond)

	lowJob := &Job{Name: "low", Ctx: context.Background(), Handler: record("low"), Priority: PriorityLow}
	highJob := &Job{Name: "high", Ctx: context.Background(), Handler: record("high"), Priority: PriorityHigh}

	go d.Submit(context.Background(), lowJob)   //nolint:errcheck
	go d.Submit(context.Background(), highJob)  //nolint:errcheck

	time.Sleep(10 * time.Millisecond)
	close(blocker)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d.Shutdown(shutdownCtx) //nolint:errcheck

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 {
		t.Fatalf("expected at least 2 ordered jobs, got %v", order)
	}
	if order[0] != "high" {
		t.Errorf("expected high to run before low, got order: %v", order)
	}
}

func TestGroupLimit(t *testing.T) {
	// Group "g" limited to 1 concurrent. Two jobs in group g submitted;
	// second must not start before first finishes.
	d := New(Config{
		Workers:     4,
		QueueSize:   100,
		GroupLimits: map[string]int{"g": 1},
	})

	var concurrent int64
	var maxConcurrent int64

	slowGroupHandler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		cur := atomic.AddInt64(&concurrent, 1)
		// Track max
		for {
			prev := atomic.LoadInt64(&maxConcurrent)
			if cur <= prev {
				break
			}
			if atomic.CompareAndSwapInt64(&maxConcurrent, prev, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt64(&concurrent, -1)
		return registry.MakeTextResult("ok"), nil
	}

	ctx := context.Background()
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			j := &Job{Name: "g", Ctx: ctx, Handler: slowGroupHandler, Group: "g"}
			d.Submit(ctx, j) //nolint:errcheck
			done <- struct{}{}
		}()
	}

	<-done
	<-done

	if atomic.LoadInt64(&maxConcurrent) > 1 {
		t.Errorf("group limit violated: max concurrent was %d, want <=1", maxConcurrent)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	d.Shutdown(shutdownCtx) //nolint:errcheck
}

func TestShutdown_graceful(t *testing.T) {
	d := New(Config{Workers: 2})

	submitted := 5
	results := make(chan *registry.CallToolResult, submitted)
	for i := 0; i < submitted; i++ {
		go func() {
			r, _ := d.Submit(context.Background(), newJob("t", makeSlowHandler(10*time.Millisecond, "ok")))
			results <- r
		}()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestShutdown_timeout(t *testing.T) {
	d := New(Config{Workers: 1})

	blocker := make(chan struct{}) // never closed
	go d.Submit(context.Background(), newJob("inf", func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) { //nolint:errcheck
		<-blocker
		return registry.MakeTextResult("never"), nil
	}))

	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := d.Shutdown(ctx)
	if err == nil {
		t.Error("expected timeout error from Shutdown")
	}
	close(blocker) // unblock to avoid goroutine leak in test
}

func TestPriorityFunc(t *testing.T) {
	pf := func(name string, td registry.ToolDefinition) Priority {
		if name == "important" {
			return PriorityCritical
		}
		return PriorityLow
	}

	d := New(Config{Workers: 1, QueueSize: 100, PriorityFunc: pf})

	order := make([]string, 0, 2)
	var mu sync.Mutex
	record := func(name string) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return registry.MakeTextResult(name), nil
		}
	}

	blocker := make(chan struct{})
	go d.Submit(context.Background(), newJob("block", func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) { //nolint:errcheck
		<-blocker
		return registry.MakeTextResult("block"), nil
	}))
	time.Sleep(10 * time.Millisecond)

	go d.Submit(context.Background(), &Job{Name: "normal", Ctx: context.Background(), Handler: record("normal")})    //nolint:errcheck
	go d.Submit(context.Background(), &Job{Name: "important", Ctx: context.Background(), Handler: record("important")}) //nolint:errcheck
	time.Sleep(10 * time.Millisecond)

	close(blocker)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d.Shutdown(shutdownCtx) //nolint:errcheck

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 {
		t.Fatalf("expected 2 completed jobs, got %v", order)
	}
	if order[0] != "important" {
		t.Errorf("PriorityFunc not applied: expected important first, got %v", order)
	}
}

func TestMiddleware_Integration(t *testing.T) {
	d := New(Config{Workers: 2, QueueSize: 100})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)

	handler := makeHandler("middleware-ok")
	td := registry.ToolDefinition{
		RuntimeGroup: "test-group",
	}
	wrapped := mw("test-tool", td, handler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok || text != "middleware-ok" {
		t.Errorf("expected 'middleware-ok', got %q", text)
	}
}

func TestMiddleware_GroupFromDefinition(t *testing.T) {
	d := New(Config{
		Workers:    4,
		QueueSize:  100,
		GroupLimits: map[string]int{"api": 1},
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)

	var concurrent int64
	var maxConcurrent int64

	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		cur := atomic.AddInt64(&concurrent, 1)
		for {
			prev := atomic.LoadInt64(&maxConcurrent)
			if cur <= prev || atomic.CompareAndSwapInt64(&maxConcurrent, prev, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&concurrent, -1)
		return registry.MakeTextResult("ok"), nil
	}

	td := registry.ToolDefinition{RuntimeGroup: "api"}
	wrapped := mw("api-tool", td, handler)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapped(context.Background(), registry.CallToolRequest{}) //nolint:errcheck
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&maxConcurrent) > 1 {
		t.Errorf("middleware group limit violated: max concurrent was %d, want <=1", maxConcurrent)
	}
}
