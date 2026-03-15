package auth

import (
	"sync"
	"time"
)

// jtiCache provides replay protection for DPoP proof JTI values.
type jtiCache struct {
	mu       sync.Mutex
	entries  map[string]time.Time
	maxSize  int
	ttl      time.Duration
	checkCtr int
}

func newJTICache(maxSize int, ttl time.Duration) *jtiCache {
	return &jtiCache{
		entries: make(map[string]time.Time),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Check returns true if the jti has NOT been seen before (and records it).
// Returns false if the jti is a replay.
func (c *jtiCache) Check(jti string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Piggyback cleanup every 100 calls
	c.checkCtr++
	if c.checkCtr%100 == 0 {
		c.cleanup(now)
	}

	if _, exists := c.entries[jti]; exists {
		return false
	}

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize {
		c.cleanup(now)
		if len(c.entries) >= c.maxSize {
			c.evictOldest()
		}
	}

	c.entries[jti] = now
	return true
}

func (c *jtiCache) cleanup(now time.Time) {
	for jti, ts := range c.entries {
		if now.Sub(ts) > c.ttl {
			delete(c.entries, jti)
		}
	}
}

func (c *jtiCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.Before(oldestTime) {
			oldestKey = k
			oldestTime = v
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}
