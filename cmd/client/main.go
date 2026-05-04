package main

import (
	"context"
	"log"
	"time"

	pb "github.com/suryaparua-official/Distributed-Rate-Limiter-API-Gateway/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewRateLimiterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send 5 requests — should all pass
	for i := 1; i <= 5; i++ {
		resp, err := client.CheckLimit(ctx, &pb.CheckLimitRequest{
			Key:       "user123",
			LimitType: "user",
			Cost:      1,
		})
		if err != nil {
			log.Fatalf("RPC failed: %v", err)
		}
		log.Printf("Request %d → allowed=%v count=%d/%d",
			i, resp.Allowed, resp.CurrentCount, resp.Limit)
	}
}