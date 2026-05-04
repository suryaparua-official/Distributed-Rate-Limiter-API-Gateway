package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestSlidingWindow_BasicAllow(t *testing.T) {
	sw := NewSlidingWindow(5, time.Second)

	for i := 0; i < 5; i++ {
		if !sw.Allow() {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if sw.Allow() {
		t.Fatal("6th request should be denied")
	}
}

func TestSlidingWindow_WindowSlides(t *testing.T) {
	sw := NewSlidingWindow(3, 200*time.Millisecond)

	for i := 0; i < 3; i++ {
		sw.Allow()
	}
	if sw.Allow() {
		t.Fatal("should be denied")
	}

	// Wait for window to slide
	time.Sleep(250 * time.Millisecond)

	if !sw.Allow() {
		t.Fatal("should be allowed after window slides")
	}
}

func TestSlidingWindow_ConcurrentSafe(t *testing.T) {
	sw := NewSlidingWindow(500, time.Second)
	var wg sync.WaitGroup
	allowed, denied := 0, 0
	var mu sync.Mutex

	for i := 0; i < 800; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := sw.Allow()
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

	if allowed > 500 {
		t.Fatalf("allowed %d, exceeds limit 500", allowed)
	}
	t.Logf("Allowed: %d, Denied: %d", allowed, denied)
}

func BenchmarkSlidingWindow_Allow(b *testing.B) {
	sw := NewSlidingWindow(1_000_000, time.Second)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sw.Allow()
		}
	})
}