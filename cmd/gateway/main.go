package main

import (
	"log"
	"os"
	"strings"

	"github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/internal/gateway"
)

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func main() {
	// Multiple gRPC nodes — comma separated
	grpcAddrsStr := getEnv("GRPC_ADDRS", "localhost:50051")
	grpcAddrs := strings.Split(grpcAddrsStr, ",")
	upstreamURL := getEnv("UPSTREAM_URL", "http://localhost:9999")
	httpPort := getEnv("HTTP_PORT", "8081")

	gw, err := gateway.NewGateway(grpcAddrs, upstreamURL)
	if err != nil {
		log.Fatalf("failed to create gateway: %v", err)
	}

	if err := gw.Start(":" + httpPort); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}