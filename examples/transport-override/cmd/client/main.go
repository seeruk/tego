package main

import (
	"context"
	"fmt"
	"log"

	"github.com/seeruk/tego/examples/transport-override/override"
	"github.com/seeruk/tego/examples/transport-override/overridepbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := overridepbv1.NewProfileServiceClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-example-request", "client")

	var header metadata.MD
	response, err := client.GetProfile(ctx, new(overridepbv1.GetProfileRequest), grpc.Header(&header))
	if err != nil {
		log.Fatal(err)
	}

	adapted := override.GetProfileResponseFromProto(response)
	fmt.Println(adapted.Profile.DisplayName)
	fmt.Println(header.Get("x-example-response"))

	tegoClient := override.NewProfileServiceGRPCClient(client)

	_, err = tegoClient.DeleteProfile(ctx, "example")
	if err != nil {
		fmt.Println(err)
		return
	}
}
