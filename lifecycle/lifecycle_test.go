package lifecycle

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	m := New()
	if m.Status() != "created" {
		t.Fatalf("expected 'created', got %q", m.Status())
	}
	if m.config.DrainTimeout != 30*time.Second {
		t.Fatalf("expected 30s drain timeout, got %v", m.config.DrainTimeout)
	}
}

func TestRun_ServeReturns(t *testing.T) {
	m := New(Config{DrainTimeout: time.Second})

	var statuses []string
	m.config.OnHealthy = func() { statuses = append(statuses, "healthy") }
	m.config.OnDraining = func() { statuses = append(statuses, "draining") }

	err := m.Run(context.Background(), func(ctx context.Context) error {
		return nil // return immediately
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status() != "stopped" {
		t.Fatalf("expected 'stopped', got %q", m.Status())
	}
	if len(statuses) != 2 || statuses[0] != "healthy" || statuses[1] != "draining" {
		t.Fatalf("unexpected status transitions: %v", statuses)
	}
}

func TestRun_ShutdownHooks_LIFO(t *testing.T) {
	m := New(Config{DrainTimeout: time.Second})

	var order []int
	m.OnShutdown(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	m.OnShutdown(func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})
	m.OnShutdown(func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	_ = m.Run(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if len(order) != 3 || order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Fatalf("expected LIFO [3,2,1], got %v", order)
	}
}

func TestRun_Signal(t *testing.T) {
	m := New(Config{
		DrainTimeout: time.Second,
		Signals:      []os.Signal{syscall.SIGUSR1},
	})

	done := make(chan error, 1)
	go func() {
		done <- m.Run(context.Background(), func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	// Wait for healthy status
	for i := 0; i < 100; i++ {
		if m.Status() == "healthy" {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Send signal
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	select {
	case err := <-done:
		// context.Canceled from serve is expected
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}

	if m.Status() != "stopped" {
		t.Fatalf("expected 'stopped', got %q", m.Status())
	}
}

func TestRun_DrainTimeout(t *testing.T) {
	m := New(Config{DrainTimeout: 100 * time.Millisecond})

	m.OnShutdown(func(ctx context.Context) error {
		<-ctx.Done() // block until drain timeout
		return ctx.Err()
	})

	start := time.Now()
	err := m.Run(context.Background(), func(ctx context.Context) error {
		return nil
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from timed-out hook")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("drain took too long: %v", elapsed)
	}
}

func TestRun_ServeError(t *testing.T) {
	m := New(Config{DrainTimeout: time.Second})

	err := m.Run(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("listen failed")
	})
	if err == nil || err.Error() != "listen failed" {
		t.Fatalf("expected 'listen failed', got %v", err)
	}
}

func TestRun_ContextCancel(t *testing.T) {
	m := New(Config{DrainTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- m.Run(ctx, func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	// Wait for healthy
	for i := 0; i < 100; i++ {
		if m.Status() == "healthy" {
			break
		}
		time.Sleep(time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	if m.Status() != "stopped" {
		t.Fatalf("expected 'stopped', got %q", m.Status())
	}
}
