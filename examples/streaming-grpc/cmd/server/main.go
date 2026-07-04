package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"net"

	"github.com/seeruk/tego/examples/streaming-grpc/streaming"
	"google.golang.org/grpc"
)

type eventService struct {
	streaming.UnimplementedEventService
}

func (eventService) Watch(ctx context.Context, topic string) (iter.Seq2[streaming.Event, error], error) {
	return func(yield func(streaming.Event, error) bool) {
		for i := range 3 {
			event := streaming.Event{
				Topic:   topic,
				Message: fmt.Sprintf("event %d", i+1),
			}
			if !yield(event, nil) {
				return
			}
		}
	}, nil
}

func (eventService) Import(ctx context.Context, events iter.Seq2[streaming.Event, error]) (int32, error) {
	var count int32
	for _, err := range events {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (eventService) Chat(ctx context.Context, events iter.Seq2[streaming.Event, error]) (iter.Seq2[streaming.Event, error], error) {
	return func(yield func(streaming.Event, error) bool) {
		for event, err := range events {
			if err != nil {
				yield(streaming.Event{}, err)
				return
			}
			event.Message = "echo: " + event.Message
			if !yield(event, nil) {
				return
			}
		}
	}, nil
}

func main() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	streaming.RegisterEventServiceGRPCServer(server, eventService{})

	log.Printf("serving streaming gRPC example on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
