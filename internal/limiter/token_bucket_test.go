package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_BasicAllow(t *testing.T) {
	// 10 capacity, 1 token/sec refill
	tb := NewTokenBucket(10, 1)

	// First 10 requests should pass (bucket starts full)
	for i := 0; i < 10; i++ {
		if !tb.Allow(1) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 11th request should be denied (bucket empty)
	if tb.Allow(1) {
		t.Fatal("should be denied when bucket is empty")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	// 5 capacity, 10 tokens/sec refill
	tb := NewTokenBucket(5, 10)

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow(1)
	}

	// Wait 500ms → should get ~5 tokens back (10/sec * 0.5s)
	time.Sleep(500 * time.Millisecond)

	if !tb.Allow(1) {
		t.Fatal("should be allowed after refill")
	}
}

func TestTokenBucket_ConcurrentSafe(t *testing.T) {
	tb := NewTokenBucket(1000, 100)
	
	var wg sync.WaitGroup
	allowed, denied := 0, 0
	var mu sync.Mutex

	// Simulate 500 concurrent requests
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := tb.Allow(1)
			mu.Lock()
			if result {
				allowed++
			} else {
				denied++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	
	// Total allowed should not exceed capacity
	if allowed > 1000 {
		t.Fatalf("allowed %d requests, exceeds capacity 1000", allowed)
	}
	t.Logf("Allowed: %d, Denied: %d", allowed, denied)
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	tb := NewTokenBucket(float64(b.N), float64(b.N))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tb.Allow(1)
		}
	})
}