package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/seeruk/tego/examples/quickstart/hello"
	"google.golang.org/grpc"
)

type greeter struct {
	hello.UnimplementedGreeterService
}

func (greeter) SayHello(ctx context.Context, name string) (string, error) {
	return fmt.Sprintf("hello, %s", name), nil
}

func main() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	hello.RegisterGreeterServiceGRPCServer(server, greeter{})

	log.Printf("serving gRPC quickstart on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
