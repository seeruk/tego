package main

import (
	"errors"
	"log"
	"net"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/hooks/catalog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:50056")
	if err != nil {
		log.Fatal(err)
	}

	adapter := catalog.NewCatalogServiceGRPCAdapter(
		catalog.Catalog{},
		tego.WithErrorMapper(grpcError),
	).AddServiceHooks(catalog.ServiceHooks()).
		AddInterfaceHooks(catalog.InterfaceHooks())

	server := grpc.NewServer()
	catalog.RegisterCatalogServiceGRPCServerWithAdapter(server, adapter)

	log.Printf("serving gRPC hooks example on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func grpcError(err error) error {
	var invalidID InvalidID
	if errors.As(err, &invalidID) {
		return status.Error(codes.InvalidArgument, invalidID.Error())
	}
	if errors.Is(err, catalog.ErrBookNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	var invalidResponse catalog.InvalidBookResponseError
	if errors.As(err, &invalidResponse) {
		return status.Error(codes.Internal, invalidResponse.Error())
	}
	return tego.GRPCError(err)
}

type InvalidID = catalog.InvalidBookIDError
