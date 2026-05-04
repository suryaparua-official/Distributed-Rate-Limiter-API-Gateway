package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/internal/ratelimiter"
	pb "github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/proto/gen"
)

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func main() {
	grpcPort := getEnv("GRPC_PORT", "50051")
	metricsPort := getEnv("METRICS_PORT", "9090")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT", "100"))
	windowDuration := time.Minute

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		})
		log.Printf("Metrics server listening on :%s", metricsPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%s", metricsPort), mux); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	rateLimiterServer := ratelimiter.NewServer(redisAddr, rateLimit, windowDuration)
	pb.RegisterRateLimiterServiceServer(grpcServer, rateLimiterServer)
	reflection.Register(grpcServer)

	log.Printf("gRPC server listening on :%s | Redis: %s | Limit: %d/min",
		grpcPort, redisAddr, rateLimit)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down gRPC server...")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}