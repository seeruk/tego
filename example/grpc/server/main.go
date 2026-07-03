package main

import (
	"log"
	"net"

	"github.com/seeruk/tego/example/yira"
	"google.golang.org/grpc"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	yira.RegisterTicketServiceGRPCServer(server, yira.NewInMemoryTicketService())

	log.Printf("serving Yira gRPC example on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
