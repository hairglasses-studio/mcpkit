package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetOrFetch(t *testing.T) {
	c := NewCache[string](100 * time.Millisecond)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "hello", nil
	}

	v, err := c.GetOrFetch(context.Background(), fetch)
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch call, got %d", calls.Load())
	}

	// Second call should use cache
	v, err = c.GetOrFetch(context.Background(), fetch)
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 fetch call (cached), got %d", calls.Load())
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache[int](10 * time.Millisecond)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (int, error) {
		n := calls.Add(1)
		return int(n), nil
	}

	v, _ := c.GetOrFetch(context.Background(), fetch)
	if v != 1 {
		t.Fatalf("got %d, want 1", v)
	}

	time.Sleep(15 * time.Millisecond)

	v, _ = c.GetOrFetch(context.Background(), fetch)
	if v != 2 {
		t.Fatalf("got %d, want 2", v)
	}
}

func TestCacheError(t *testing.T) {
	c := NewCache[string](100 * time.Millisecond)
	errBad := errors.New("fetch failed")

	v, err := c.GetOrFetch(context.Background(), func(ctx context.Context) (string, error) {
		return "", errBad
	})
	if !errors.Is(err, errBad) {
		t.Fatalf("expected errBad, got %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty string, got %q", v)
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := NewCache[string](time.Hour)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "data", nil
	}

	c.GetOrFetch(context.Background(), fetch)
	if calls.Load() != 1 {
		t.Fatal("expected 1 call")
	}

	c.Invalidate()

	c.GetOrFetch(context.Background(), fetch)
	if calls.Load() != 2 {
		t.Fatal("expected 2 calls after invalidation")
	}
}

func TestCacheGet(t *testing.T) {
	c := NewCache[string](time.Hour)

	_, ok := c.Get()
	if ok {
		t.Fatal("expected false for empty cache")
	}

	c.Set("value")
	v, ok := c.Get()
	if !ok || v != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", v, ok)
	}
}

func TestCacheSet(t *testing.T) {
	c := NewCache[int](time.Hour)
	c.Set(42)

	v, err := c.GetOrFetch(context.Background(), func(ctx context.Context) (int, error) {
		t.Fatal("should not be called when cache is set")
		return 0, nil
	})
	if err != nil || v != 42 {
		t.Fatalf("expected (42, nil), got (%d, %v)", v, err)
	}
}

func TestConcurrentGetOrFetch(t *testing.T) {
	c := NewCache[int](time.Hour)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (int, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return 99, nil
	}

	done := make(chan int, 10)
	for range 10 {
		go func() {
			v, _ := c.GetOrFetch(context.Background(), fetch)
			done <- v
		}()
	}

	for range 10 {
		v := <-done
		if v != 99 {
			t.Fatalf("got %d, want 99", v)
		}
	}

	if n := calls.Load(); n != 1 {
		t.Fatalf("expected 1 fetch (singleflight), got %d", n)
	}
}
