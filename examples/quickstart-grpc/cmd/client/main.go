package main

import (
	"context"
	"fmt"
	"log"

	"github.com/seeruk/tego/examples/quickstart/hello"
	"github.com/seeruk/tego/examples/quickstart/hellopbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := hello.NewGreeterServiceGRPCClient(hellopbv1.NewGreeterServiceClient(conn))

	message, err := client.SayHello(context.Background(), "Tego")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(message)
}
