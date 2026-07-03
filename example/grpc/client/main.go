package main

import (
	"context"
	"log"
	"time"

	"github.com/seeruk/tego/example/workflow"
	"github.com/seeruk/tego/example/yira"
	"github.com/seeruk/tego/example/yirapbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("close gRPC connection: %v", err)
		}
	}()

	nativeClient := yirapbv1.NewTicketServiceClient(conn)
	client := yira.NewTicketServiceGRPCClient(nativeClient)

	if err := workflow.Run(ctx, "grpc", client); err != nil {
		log.Fatal(err)
	}
}
