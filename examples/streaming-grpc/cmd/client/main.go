package main

import (
	"context"
	"fmt"
	"iter"
	"log"

	"github.com/seeruk/tego/examples/streaming-grpc/streaming"
	"github.com/seeruk/tego/examples/streaming-grpc/streamingpbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := streaming.NewEventServiceGRPCClient(streamingpbv1.NewEventServiceClient(conn))
	ctx := context.Background()

	events, err := client.Watch(ctx, "builds")
	if err != nil {
		log.Fatal(err)
	}
	for event, err := range events {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(event.Message)
	}

	count, err := client.Import(ctx, sampleEvents())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("imported %d events\n", count)

	replies, err := client.Chat(ctx, sampleEvents())
	if err != nil {
		log.Fatal(err)
	}
	for reply, err := range replies {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(reply.Message)
	}
}

func sampleEvents() iter.Seq2[streaming.Event, error] {
	return func(yield func(streaming.Event, error) bool) {
		for i := range 2 {
			event := streaming.Event{
				Topic:   "builds",
				Message: fmt.Sprintf("client event %d", i+1),
			}
			if !yield(event, nil) {
				return
			}
		}
	}
}
