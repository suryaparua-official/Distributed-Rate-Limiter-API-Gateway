package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	pb "github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	rateLimiter pb.RateLimiterServiceClient
	proxy       *httputil.ReverseProxy
	upstream    string
}

func NewGateway(grpcAddr, upstreamURL string) (*Gateway, error) {
	conn, err := grpc.NewClient(grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	target, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	// Inject rate limit headers into upstream response
	proxy.ModifyResponse = func(resp *http.Response) error {
		if v := resp.Request.Header.Get("X-Internal-RL-Limit"); v != "" {
			resp.Header.Set("X-RateLimit-Limit", v)
			resp.Header.Set("X-RateLimit-Remaining", resp.Request.Header.Get("X-Internal-RL-Remaining"))
			resp.Header.Set("X-RateLimit-Reset", resp.Request.Header.Get("X-Internal-RL-Reset"))
		}
		return nil
	}

	return &Gateway{
		rateLimiter: pb.NewRateLimiterServiceClient(conn),
		proxy:       proxy,
		upstream:    upstreamURL,
	}, nil
}

func extractKey(r *http.Request) (string, string) {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey, "api_key"
	}
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}
	return ip, "ip"
}

func (g *Gateway) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Millisecond)
		defer cancel()

		key, limitType := extractKey(r)

		resp, err := g.rateLimiter.CheckLimit(ctx, &pb.CheckLimitRequest{
			Key:       key,
			LimitType: limitType,
			Cost:      1,
		})

		if err != nil {
			log.Printf("rate limiter error: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		// Pass rate limit info via internal request headers → ModifyResponse picks them up
		r.Header.Set("X-Internal-RL-Limit", fmt.Sprintf("%d", resp.Limit))
		r.Header.Set("X-Internal-RL-Remaining", fmt.Sprintf("%d", resp.Limit-resp.CurrentCount))
		r.Header.Set("X-Internal-RL-Reset", fmt.Sprintf("%d", resp.ResetAfterMs))

		if !resp.Allowed {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", resp.Limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", resp.ResetAfterMs/1000))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate_limit_exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.proxy.ServeHTTP(w, r)
}

func (g *Gateway) Start(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/", g.RateLimitMiddleware(g))

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("HTTP Gateway listening on %s → upstream: %s", addr, g.upstream)
	return server.ListenAndServe()
}