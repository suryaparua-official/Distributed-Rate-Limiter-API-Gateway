# Distributed Rate Limiter + API Gateway

Production-grade distributed rate limiting system built with Go, gRPC, Redis, and Docker.

## Architecture

```
Client → HTTP Gateway (:8081)
              ↓ Rate Limit Middleware
         gRPC Rate Limiter (:50051)
              ↓ Lua Script (atomic)
         Redis Cluster (:6379)
              ↓ allow/deny
         Upstream Service
```

## Tech Stack

| Component     | Technology              |
| ------------- | ----------------------- |
| Language      | Go 1.25                 |
| API Gateway   | Go HTTP + Reverse Proxy |
| Rate Limiter  | Redis + Lua Scripts     |
| Communication | gRPC + Protobuf         |
| Observability | Prometheus              |
| Container     | Docker + Docker Compose |
| Load Testing  | k6                      |

## Algorithms Benchmark

| Algorithm      | Latency    | req/sec | Use Case                   |
| -------------- | ---------- | ------- | -------------------------- |
| Token Bucket   | 65 ns/op   | ~15M    | Single node, burst traffic |
| Sliding Window | 65 µs/op   | ~15K    | Accurate limiting          |
| Redis Lua      | ~130 µs/op | ~7K     | Distributed, multi-node    |

## Quick Start

### Run with Docker

```bash
# Start full stack
docker compose up -d

# Verify services
docker compose ps
```

### Run Locally

```bash
# Start Redis
docker compose up -d redis

# Start gRPC rate limiter
go run ./cmd/server

# Start HTTP gateway
go run ./cmd/gateway
```

## API

### Health Check

```
GET http://localhost:8081/health
Response: {"status":"ok"}
```

### Proxied Request (Rate Limited)

```
GET http://localhost:8081/get
Headers:
  X-API-Key: user123
```

### Response Headers

```
X-RateLimit-Limit:     100
X-RateLimit-Remaining: 99
X-RateLimit-Reset:     60000
```

### Rate Limited Response (429)

```json
{ "error": "rate_limit_exceeded" }
```

## Project Structure

```
├── cmd/
│   ├── server/           # gRPC rate limiter server
│   ├── gateway/          # HTTP API gateway
│   └── client/           # Test gRPC client
├── internal/
│   ├── limiter/          # Core algorithms
│   │   ├── token_bucket.go
│   │   ├── sliding_window.go
│   │   ├── redis_limiter.go
│   │   ├── circuit_breaker.go
│   │   └── multi_tier.go
│   ├── gateway/          # Gateway + consistent hashing
│   ├── ratelimiter/      # gRPC server implementation
│   └── metrics/          # Prometheus metrics
├── proto/                # Protobuf definitions
├── tests/                # k6 load tests
├── Dockerfile.server
├── Dockerfile.gateway
├── docker-compose.yml
└── prometheus.yml
```

## Configuration

| Environment Variable | Default          | Description             |
| -------------------- | ---------------- | ----------------------- |
| `REDIS_ADDR`         | `localhost:6379` | Redis address           |
| `GRPC_PORT`          | `50051`          | gRPC server port        |
| `METRICS_PORT`       | `9090`           | Prometheus metrics port |
| `RATE_LIMIT`         | `100`            | Requests per window     |

## Rate Limiting Tiers

| Tier   | Default Limit | Window   |
| ------ | ------------- | -------- |
| IP     | 100 req       | 1 minute |
| User   | 1000 req      | 1 minute |
| Global | 100,000 req   | 1 minute |

## Observability

### Prometheus Metrics

```
# Rate limit decisions
ratelimiter_decisions_total{decision="allowed|denied", limit_type="ip|user|global"}

# Request latency
ratelimiter_request_duration_seconds{method="CheckLimit"}

# Circuit breaker state (0=closed, 1=open, 2=half-open)
ratelimiter_circuit_breaker_state{}

# Redis errors
ratelimiter_redis_errors_total{error_type="timeout|circuit_open|connection"}
```

### Service Endpoints

| Service            | URL                           |
| ------------------ | ----------------------------- |
| HTTP Gateway       | http://localhost:8081         |
| gRPC Server        | localhost:50051               |
| Prometheus Metrics | http://localhost:9090/metrics |
| Prometheus UI      | http://localhost:9091         |

## Testing

```bash
# Unit tests
go test ./...

# Benchmark
go test -bench=. -benchmem ./internal/limiter/

# Load test
k6 run tests/load_test.js
```

## Benchmark Results

```
BenchmarkTokenBucket_Allow-8      45,916,492      65 ns/op     0 B/op    0 allocs/op
BenchmarkSlidingWindow_Allow-8       136,651   65,060 ns/op     0 B/op    0 allocs/op
BenchmarkRedisLimiter_Allow-8         25,922  129,920 ns/op   690 B/op   22 allocs/op
```

## Key Design Decisions

**Why Token Bucket?**
O(1) time complexity, supports burst traffic, no background goroutines needed.

**Why Lua Scripts in Redis?**
Atomic execution — no race conditions possible across distributed nodes.

**Why gRPC?**
Binary serialization ~5x faster than JSON, built-in load balancing, streaming support.

**Why Circuit Breaker?**
Prevents cascade failures when Redis is unavailable — fail-open keeps service running.

**Why Consistent Hashing?**
Adding/removing nodes only remaps ~1/N keys instead of all keys.

**Why Multi-tier Rate Limiting?**
IP limits prevent abuse, user limits enforce quotas, global limits protect the system.

## Load Test Results (k6)

```
Scenarios:
  normal_load: 10 VUs for 30s
  spike:       up to 200 VUs for 40s

Results:
  Total requests:  ~38327
  Throughput:      ~510 req/s
  avg latency:     4.9ms
  p(95) latency:   9.7ms ← within <10ms SLA
  max latency: 452ms (spike peak)
```

### With External Upstream (httpbin.org)

```
Throughput: 86req/s
avg latency: 421ms
p(95): 956ms
```

## Author

**Surya** — Built as a production-grade distributed systems project.

Inspired by rate limiting systems at Google, Meta, and Cloudflare.
