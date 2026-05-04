package limiter

import (
	"context"
	"fmt"
	"time"
)

// TierConfig defines limits for each tier
type TierConfig struct {
	IPLimit     int
	IPWindow    time.Duration
	UserLimit   int
	UserWindow  time.Duration
	GlobalLimit int
	GlobalWindow time.Duration
}

// DefaultTierConfig — production-grade defaults
func DefaultTierConfig() TierConfig {
	return TierConfig{
		IPLimit:      100,
		IPWindow:     time.Minute,
		UserLimit:    1000,
		UserWindow:   time.Minute,
		GlobalLimit:  100000,
		GlobalWindow: time.Minute,
	}
}

// MultiTierLimiter checks IP → User → Global limits in sequence.
// All three must pass for request to be allowed.
type MultiTierLimiter struct {
	ipLimiter     *RedisLimiter
	userLimiter   *RedisLimiter
	globalLimiter *RedisLimiter
}

func NewMultiTierLimiter(redisAddr string, cfg TierConfig) *MultiTierLimiter {
	return &MultiTierLimiter{
		ipLimiter:     NewRedisLimiter(redisAddr, cfg.IPLimit, cfg.IPWindow),
		userLimiter:   NewRedisLimiter(redisAddr, cfg.UserLimit, cfg.UserWindow),
		globalLimiter: NewRedisLimiter(redisAddr, cfg.GlobalLimit, cfg.GlobalWindow),
	}
}

// CheckResult holds the result of a multi-tier check
type CheckResult struct {
	Allowed      bool
	DeniedTier   string // which tier denied: "ip", "user", "global"
	IPCount      int
	UserCount    int
	GlobalCount  int
}

// Allow checks all three tiers.
// Short-circuits on first denial — faster and avoids unnecessary Redis calls.
func (m *MultiTierLimiter) Allow(ctx context.Context, ip, userID string) (CheckResult, error) {
	result := CheckResult{Allowed: true}

	// Tier 1: IP check (strictest — protects against abuse)
	ipAllowed, ipCount, err := m.ipLimiter.Allow(ctx, fmt.Sprintf("ip:%s", ip))
	result.IPCount = ipCount
	if err != nil {
		return result, fmt.Errorf("ip tier error: %w", err)
	}
	if !ipAllowed {
		result.Allowed = false
		result.DeniedTier = "ip"
		return result, nil // short-circuit
	}

	// Tier 2: User check (per authenticated user)
	if userID != "" {
		userAllowed, userCount, err := m.userLimiter.Allow(ctx, fmt.Sprintf("user:%s", userID))
		result.UserCount = userCount
		if err != nil {
			return result, fmt.Errorf("user tier error: %w", err)
		}
		if !userAllowed {
			result.Allowed = false
			result.DeniedTier = "user"
			return result, nil
		}
	}

	// Tier 3: Global check (system-wide protection)
	globalAllowed, globalCount, err := m.globalLimiter.Allow(ctx, "global:system")
	result.GlobalCount = globalCount
	if err != nil {
		return result, fmt.Errorf("global tier error: %w", err)
	}
	if !globalAllowed {
		result.Allowed = false
		result.DeniedTier = "global"
		return result, nil
	}

	return result, nil
}