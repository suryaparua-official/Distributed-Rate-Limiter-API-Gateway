package limiter

import (
	"sync"
	"time"
)

// SlidingWindow tracks request timestamps within a rolling time window.
// More memory-intensive than Token Bucket but eliminates boundary spikes.
type SlidingWindow struct {
	mu         sync.Mutex
	windowSize time.Duration
	limit      int
	requests   []time.Time // circular buffer of request timestamps
}

func NewSlidingWindow(limit int, windowSize time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windowSize: windowSize,
		limit:      limit,
		requests:   make([]time.Time, 0, limit),
	}
}

// Allow checks if request is within limit for the current window.
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.windowSize)

	// Evict timestamps outside the window (keep only recent ones)
	valid := sw.requests[:0]
	for _, t := range sw.requests {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	sw.requests = valid

	if len(sw.requests) >= sw.limit {
		return false
	}

	sw.requests = append(sw.requests, now)
	return true
}

// Stats returns current request count in window
func (sw *SlidingWindow) Stats() (current, limit int) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return len(sw.requests), sw.limit
}