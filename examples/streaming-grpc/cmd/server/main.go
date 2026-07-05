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

func (eventService) Watch(ctx context.Context, topic string) (iter.Seq2[streaming.WatchResponse, error], error) {
	return func(yield func(streaming.WatchResponse, error) bool) {
		for i := range 3 {
			event := streaming.Event{
				Topic:   topic,
				Message: fmt.Sprintf("event %d", i+1),
			}
			if !yield(streaming.WatchResponse{Event: event}, nil) {
				return
			}
		}
	}, nil
}

func (eventService) Import(ctx context.Context, requests iter.Seq2[streaming.ImportRequest, error]) (int32, error) {
	var count int32
	for _, err := range requests {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (eventService) Chat(ctx context.Context, requests iter.Seq2[streaming.ChatRequest, error]) (iter.Seq2[streaming.ChatResponse, error], error) {
	return func(yield func(streaming.ChatResponse, error) bool) {
		for request, err := range requests {
			if err != nil {
				yield(streaming.ChatResponse{}, err)
				return
			}
			event := request.Event
			event.Message = "echo: " + event.Message
			if !yield(streaming.ChatResponse{Event: event}, nil) {
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
