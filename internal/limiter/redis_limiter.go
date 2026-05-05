package limiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client    *redis.Client
	limit     int
	window    time.Duration
	luaScript *redis.Script
	cb        *CircuitBreaker
}

const slidingWindowLua = `
local key         = KEYS[1]
local now         = tonumber(ARGV[1])
local window      = tonumber(ARGV[2])
local limit       = tonumber(ARGV[3])
local unique      = ARGV[4]
local windowStart = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', windowStart)

local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, unique)
    redis.call('PEXPIRE', key, math.ceil(window / 1000000))
    return {1, count + 1}
end

return {0, count}
`

func NewRedisLimiter(addr string, limit int, window time.Duration) *RedisLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	return &RedisLimiter{
		client:    client,
		limit:     limit,
		window:    window,
		luaScript: redis.NewScript(slidingWindowLua),
		cb: NewCircuitBreaker(
			5,
			30*time.Second,
		),
	}
}

func uniqueID(now int64) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d-%s", now, hex.EncodeToString(b))
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	redisKey := fmt.Sprintf("rl:%s", key)
	now := time.Now().UnixNano()
	windowNs := rl.window.Nanoseconds()
	member := uniqueID(now)

	var allowed bool
	var count int

	err := rl.cb.Execute(func() error {
		result, err := rl.luaScript.Run(
			ctx,
			rl.client,
			[]string{redisKey},
			now, windowNs, rl.limit, member,
		).Slice()

		if err != nil {
			return err
		}

		allowed = result[0].(int64) == 1
		count = int(result[1].(int64))
		return nil
	})

	if err != nil {
		if err == ErrCircuitOpen {
			return true, 0, ErrCircuitOpen
		}
		return true, 0, fmt.Errorf("redis error: %w", err)
	}

	return allowed, count, nil
}

func (rl *RedisLimiter) Close() error {
	return rl.client.Close()
}

// GetCount returns current request count without incrementing.
func (rl *RedisLimiter) GetCount(ctx context.Context, redisKey string) (int, error) {
	now := time.Now().UnixNano()
	windowStart := now - rl.window.Nanoseconds()

	pipe := rl.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "-inf", fmt.Sprintf("%d", windowStart))
	cardCmd := pipe.ZCard(ctx, redisKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return int(cardCmd.Val()), nil
}

// GetCircuitState returns current circuit breaker state
func (rl *RedisLimiter) GetCircuitState() (State, int, time.Time) {
	return rl.cb.Stats()
}