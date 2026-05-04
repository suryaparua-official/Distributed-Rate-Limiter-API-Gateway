package limiter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func setupRedisLimiter(t *testing.T, limit int, window time.Duration) *RedisLimiter {
	t.Helper()
	rl := NewRedisLimiter("localhost:6379", limit, window)
	
	// Verify Redis connection
	ctx := context.Background()
	if err := rl.client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	return rl
}

func TestRedisLimiter_BasicAllow(t *testing.T) {
	rl := setupRedisLimiter(t, 5, time.Second)
	defer rl.Close()

	ctx := context.Background()
	key := fmt.Sprintf("test:basic:%d", time.Now().UnixNano()) // unique key per test

	for i := 0; i < 5; i++ {
		allowed, count, err := rl.Allow(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed, count=%d", i+1, count)
		}
	}

	// 6th request must be denied
	allowed, count, err := rl.Allow(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("6th request should be denied, count=%d", count)
	}
}

func TestRedisLimiter_WindowExpiry(t *testing.T) {
	rl := setupRedisLimiter(t, 3, 500*time.Millisecond)
	defer rl.Close()

	ctx := context.Background()
	key := fmt.Sprintf("test:expiry:%d", time.Now().UnixNano())

	// Fill the window
	for i := 0; i < 3; i++ {
		rl.Allow(ctx, key)
	}

	// Should be denied
	allowed, _, _ := rl.Allow(ctx, key)
	if allowed {
		t.Fatal("should be denied when limit reached")
	}

	// Wait for window to expire
	time.Sleep(600 * time.Millisecond)

	// Should be allowed again
	allowed, _, err := rl.Allow(ctx, key)
	if err != nil || !allowed {
		t.Fatalf("should be allowed after window expires: err=%v, allowed=%v", err, allowed)
	}
}

func TestRedisLimiter_Concurrent(t *testing.T) {
	rl := setupRedisLimiter(t, 100, 5*time.Second)
	defer rl.Close()

	ctx := context.Background()
	key := fmt.Sprintf("test:concurrent:%d", time.Now().UnixNano())

	var wg sync.WaitGroup
	var allowedCount, deniedCount int64
	var mu sync.Mutex

	// 200 concurrent requests — only 100 should pass
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _ := rl.Allow(ctx, key)
			mu.Lock()
			if allowed {
				allowedCount++
			} else {
				deniedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	t.Logf("Allowed: %d, Denied: %d", allowedCount, deniedCount)

	if allowedCount > 100 {
		t.Fatalf("allowed %d requests, exceeds limit 100", allowedCount)
	}
}

func BenchmarkRedisLimiter_Allow(b *testing.B) {
	rl := NewRedisLimiter("localhost:6379", 1_000_000, time.Minute)
	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		key := fmt.Sprintf("bench:%d", time.Now().UnixNano())
		for pb.Next() {
			rl.Allow(ctx, key)
		}
	})
}