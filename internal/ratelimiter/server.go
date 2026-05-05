package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/internal/limiter"
	"github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/internal/metrics"
	pb "github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/proto/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedRateLimiterServiceServer
	redisLimiter *limiter.RedisLimiter
	limit        int
	window       time.Duration
}

func NewServer(redisAddr string, limit int, window time.Duration) *Server {
	return &Server{
		redisLimiter: limiter.NewRedisLimiter(redisAddr, limit, window),
		limit:        limit,
		window:       window,
	}
}

func (s *Server) CheckLimit(ctx context.Context, req *pb.CheckLimitRequest) (*pb.CheckLimitResponse, error) {
	start := time.Now()
	defer func() {
		// Record latency for every CheckLimit call
		metrics.RequestDuration.WithLabelValues("CheckLimit").
			Observe(time.Since(start).Seconds())
	}()

	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	compositeKey := req.LimitType + ":" + req.Key
	allowed, count, err := s.redisLimiter.Allow(ctx, compositeKey)

if err != nil {
    if err == limiter.ErrCircuitOpen {
        metrics.RedisErrors.WithLabelValues("circuit_open").Inc()
        metrics.CircuitBreakerState.Set(1) // open
    } else {
        metrics.RedisErrors.WithLabelValues("connection").Inc()
    }
    return &pb.CheckLimitResponse{
        Allowed:      true,
        CurrentCount: 0,
        Limit:        int32(s.limit),
        Reason:       "redis_unavailable_fail_open",
    }, nil
}

	// Success — check actual CB state
	state, _, _ := s.redisLimiter.GetCircuitState()
	metrics.CircuitBreakerState.Set(float64(state))

	// Record allow/deny decision
	decision := "allowed"
	if !allowed {
		decision = "denied"
	}
	metrics.RateLimitDecisions.WithLabelValues(decision, req.LimitType).Inc()
	metrics.CircuitBreakerState.Set(0) // closed — Redis is healthy

	resp := &pb.CheckLimitResponse{
		Allowed:      allowed,
		CurrentCount: int32(count),
		Limit:        int32(s.limit),
		ResetAfterMs: s.window.Milliseconds(),
	}

	if !allowed {
		resp.Reason = "rate_limit_exceeded"
	}

	return resp, nil
}

func (s *Server) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	redisKey := fmt.Sprintf("rl:%s", req.Key)
	count, err := s.redisLimiter.GetCount(ctx, redisKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "redis error: %v", err)
	}

	return &pb.GetStatsResponse{
		Key:          req.Key,
		CurrentCount: int32(count),
		Limit:        int32(s.limit),
		UsagePercent: float64(count) / float64(s.limit) * 100,
	}, nil
}