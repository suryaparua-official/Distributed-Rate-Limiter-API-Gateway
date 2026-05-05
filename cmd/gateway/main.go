package main

import (
	"log"

	"github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/internal/gateway"
)

func main() {
gw, err := gateway.NewGateway(
    "localhost:50051",
    "http://localhost:9999", // local mock — no network latency
)
	if err != nil {
		log.Fatalf("failed to create gateway: %v", err)
	}

	if err := gw.Start(":8081"); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}