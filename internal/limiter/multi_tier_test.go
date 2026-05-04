package limiter

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMultiTierLimiter_AllTiersPass(t *testing.T) {
	cfg := TierConfig{
		IPLimit:      10,
		IPWindow:     time.Minute,
		UserLimit:    100,
		UserWindow:   time.Minute,
		GlobalLimit:  10000,
		GlobalWindow: time.Minute,
	}

	ml := NewMultiTierLimiter("localhost:6379", cfg)
	ctx := context.Background()

	ip := fmt.Sprintf("1.2.3.%d", time.Now().UnixNano()%255)
	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())

	result, err := ml.Allow(ctx, ip, userID)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	if !result.Allowed {
		t.Fatalf("should be allowed, denied at tier: %s", result.DeniedTier)
	}
	t.Logf("Allowed ✓ | IP count: %d | User count: %d | Global count: %d",
		result.IPCount, result.UserCount, result.GlobalCount)
}

func TestMultiTierLimiter_IPTierDenies(t *testing.T) {
	cfg := TierConfig{
		IPLimit:      3, // very low IP limit
		IPWindow:     time.Minute,
		UserLimit:    1000,
		UserWindow:   time.Minute,
		GlobalLimit:  100000,
		GlobalWindow: time.Minute,
	}

	ml := NewMultiTierLimiter("localhost:6379", cfg)
	ctx := context.Background()

	ip := fmt.Sprintf("9.9.9.%d", time.Now().UnixNano()%255)
	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())

	// Exhaust IP limit
	for i := 0; i < 3; i++ {
		ml.Allow(ctx, ip, userID)
	}

	// 4th request should be denied at IP tier
	result, err := ml.Allow(ctx, ip, userID)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	if result.Allowed {
		t.Fatal("should be denied at IP tier")
	}
	if result.DeniedTier != "ip" {
		t.Fatalf("expected denial at 'ip' tier, got: %s", result.DeniedTier)
	}
	t.Logf("Correctly denied at tier: %s ✓", result.DeniedTier)
}