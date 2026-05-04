package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RateLimitDecisions — allow/deny counter by key type
	RateLimitDecisions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_decisions_total",
			Help: "Total rate limit decisions (allowed/denied)",
		},
		[]string{"decision", "limit_type"}, // labels
	)

	// RequestDuration — gRPC CheckLimit latency histogram
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ratelimiter_request_duration_seconds",
			Help:    "Rate limiter request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		},
		[]string{"method"},
	)

	// RedisErrors — Redis/circuit breaker errors
	RedisErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_redis_errors_total",
			Help: "Total Redis errors by type",
		},
		[]string{"error_type"}, // "timeout", "circuit_open", "connection"
	)

	// CircuitBreakerState — current state (0=closed, 1=open, 2=half-open)
	CircuitBreakerState = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ratelimiter_circuit_breaker_state",
			Help: "Circuit breaker state: 0=closed, 1=open, 2=half-open",
		},
	)

	// ActiveConnections — current Redis pool connections
	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ratelimiter_redis_active_connections",
			Help: "Current active Redis connections",
		},
	)

	// GatewayRequests — HTTP gateway request counter
	GatewayRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total HTTP gateway requests",
		},
		[]string{"method", "path", "status"},
	)

	// GatewayLatency — HTTP gateway response time
	GatewayLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "HTTP gateway request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)