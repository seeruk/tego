package main

import (
	"errors"
	"log"
	"net"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/error-mapping/books"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:50052")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	books.RegisterBookServiceGRPCServer(
		server,
		books.BookStore{},
		tego.WithErrorMapper(grpcError),
	)

	log.Printf("serving gRPC error-mapping example on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func grpcError(err error) error {
	if errors.Is(err, books.ErrBookNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if invalid, ok := errors.AsType[books.InvalidBookIDError](err); ok {
		return status.Error(codes.InvalidArgument, invalid.Error())
	}
	return tego.GRPCError(err)
}
