package limiter

import (
	"sync"
	"time"
)

// TokenBucket implements the token bucket algorithm.
// Thread-safe for concurrent use.
type TokenBucket struct {
	mu           sync.Mutex
	capacity     float64   // max tokens the bucket can hold
	tokens       float64   // current token count
	refillRate   float64   // tokens added per second
	lastRefillAt time.Time // last time we refilled
}

// NewTokenBucket creates a new token bucket.
// capacity: max burst size (e.g. 100)
// refillRate: tokens per second (e.g. 10 = 10 req/s sustained)
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:     capacity,
		tokens:       capacity, // start full
		refillRate:   refillRate,
		lastRefillAt: time.Now(),
	}
}

// Allow checks if a request can proceed.
// Returns true if allowed, false if rate-limited.
// Cost parameter lets us consume multiple tokens (e.g. for heavy endpoints).
func (tb *TokenBucket) Allow(cost float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens < cost {
		return false // not enough tokens — deny
	}

	tb.tokens -= cost
	return true
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with mu held.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillAt).Seconds()
	
	// Add tokens proportional to elapsed time
	tb.tokens += elapsed * tb.refillRate
	
	// Cap at capacity — can't exceed bucket size
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	
	tb.lastRefillAt = now
}

// Stats returns current bucket state (useful for metrics/debugging)
func (tb *TokenBucket) Stats() (current, capacity float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens, tb.capacity
}