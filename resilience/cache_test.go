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

func TestKeyedCacheDifferentArgsDifferentEntries(t *testing.T) {
	c := NewKeyedCache[string](time.Hour, 10)
	calls := atomic.Int32{}

	fetch := func(want string) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			calls.Add(1)
			return want, nil
		}
	}

	keyA := CacheKey("mytool", map[string]any{"x": 1})
	keyB := CacheKey("mytool", map[string]any{"x": 2})
	if keyA == keyB {
		t.Fatalf("expected distinct keys for distinct args, got %q for both", keyA)
	}

	v, err := c.GetOrFetch(context.Background(), keyA, fetch("a"))
	if err != nil || v != "a" {
		t.Fatalf("got (%q, %v), want (a, nil)", v, err)
	}

	v, err = c.GetOrFetch(context.Background(), keyB, fetch("b"))
	if err != nil || v != "b" {
		t.Fatalf("got (%q, %v), want (b, nil)", v, err)
	}

	// Re-fetching keyA must still return "a" (not clobbered by keyB's fetch)
	// and must not have called fetchFn again.
	v, err = c.GetOrFetch(context.Background(), keyA, fetch("should-not-be-called"))
	if err != nil || v != "a" {
		t.Fatalf("got (%q, %v), want (a, nil)", v, err)
	}

	if n := calls.Load(); n != 2 {
		t.Fatalf("expected 2 fetch calls (one per distinct key), got %d", n)
	}
	if n := c.Len(); n != 2 {
		t.Fatalf("expected 2 entries, got %d", n)
	}
}

func TestKeyedCacheTTLExpiry(t *testing.T) {
	c := NewKeyedCache[int](10*time.Millisecond, 10)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (int, error) {
		n := calls.Add(1)
		return int(n), nil
	}

	v, _ := c.GetOrFetch(context.Background(), "k", fetch)
	if v != 1 {
		t.Fatalf("got %d, want 1", v)
	}

	time.Sleep(15 * time.Millisecond)

	v, _ = c.GetOrFetch(context.Background(), "k", fetch)
	if v != 2 {
		t.Fatalf("got %d, want 2 after TTL expiry", v)
	}
}

func TestKeyedCacheEvictionBound(t *testing.T) {
	c := NewKeyedCache[int](time.Hour, 3)

	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		c.GetOrFetch(context.Background(), key, func(ctx context.Context) (int, error) {
			return i, nil
		})
	}

	if n := c.Len(); n != 3 {
		t.Fatalf("expected cache bounded to 3 entries, got %d", n)
	}

	// The two oldest keys ("a", "b") should have been evicted; the three
	// most recent ("c", "d", "e") should still be cached (no re-fetch).
	calls := atomic.Int32{}
	for _, key := range []string{"c", "d", "e"} {
		c.GetOrFetch(context.Background(), key, func(ctx context.Context) (int, error) {
			calls.Add(1)
			return -1, nil
		})
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("expected recently-used entries to survive eviction, got %d re-fetches", n)
	}

	// The evicted keys should trigger a fresh fetch.
	refetches := atomic.Int32{}
	for _, key := range []string{"a", "b"} {
		c.GetOrFetch(context.Background(), key, func(ctx context.Context) (int, error) {
			refetches.Add(1)
			return -1, nil
		})
	}
	if n := refetches.Load(); n != 2 {
		t.Fatalf("expected both evicted keys to trigger a re-fetch, got %d", n)
	}
}

func TestKeyedCacheLRUTouchOnGet(t *testing.T) {
	c := NewKeyedCache[int](time.Hour, 2)

	c.GetOrFetch(context.Background(), "a", func(ctx context.Context) (int, error) { return 1, nil })
	c.GetOrFetch(context.Background(), "b", func(ctx context.Context) (int, error) { return 2, nil })

	// Touch "a" so "b" becomes the least-recently-used entry.
	c.GetOrFetch(context.Background(), "a", func(ctx context.Context) (int, error) {
		t.Fatal("should not be called; a is still fresh")
		return 0, nil
	})

	// Inserting "c" should evict "b" (LRU), not "a".
	c.GetOrFetch(context.Background(), "c", func(ctx context.Context) (int, error) { return 3, nil })

	calls := atomic.Int32{}
	c.GetOrFetch(context.Background(), "a", func(ctx context.Context) (int, error) {
		calls.Add(1)
		return -1, nil
	})
	if calls.Load() != 0 {
		t.Fatal("expected a to survive eviction (recently touched)")
	}

	refetch := atomic.Int32{}
	c.GetOrFetch(context.Background(), "b", func(ctx context.Context) (int, error) {
		refetch.Add(1)
		return -1, nil
	})
	if refetch.Load() != 1 {
		t.Fatal("expected b to have been evicted (least recently used)")
	}
}

func TestCacheKeyCanonicalizationStability(t *testing.T) {
	a := map[string]any{"z": 1, "a": 2, "m": map[string]any{"y": 1, "b": 2}}
	b := map[string]any{"a": 2, "m": map[string]any{"b": 2, "y": 1}, "z": 1}

	keyA := CacheKey("tool", a)
	keyB := CacheKey("tool", b)
	if keyA != keyB {
		t.Fatalf("expected key-order-independent hashing, got %q != %q", keyA, keyB)
	}

	// A genuinely different value must still produce a different key.
	c := map[string]any{"a": 2, "m": map[string]any{"b": 2, "y": 1}, "z": 999}
	if CacheKey("tool", c) == keyA {
		t.Fatal("expected different values to produce different keys")
	}
}

func TestKeyedCacheInvalidate(t *testing.T) {
	c := NewKeyedCache[string](time.Hour, 10)
	calls := atomic.Int32{}

	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "data", nil
	}

	c.GetOrFetch(context.Background(), "k", fetch)
	c.Invalidate("k")
	c.GetOrFetch(context.Background(), "k", fetch)

	if n := calls.Load(); n != 2 {
		t.Fatalf("expected 2 fetches after invalidation, got %d", n)
	}
}
