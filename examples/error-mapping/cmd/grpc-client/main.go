package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/error-mapping/books"
	"github.com/seeruk/tego/examples/error-mapping/bookspbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := books.NewBookServiceGRPCClient(
		bookspbv1.NewBookServiceClient(conn),
		tego.WithErrorMapper(grpcClientError),
	)
	for _, id := range []string{"tego", "missing", ""} {
		book, err := client.GetBook(context.Background(), id)
		if err != nil {
			printDomainError(id, err)
			continue
		}
		fmt.Printf("%q -> %s by %s\n", id, book.Title, book.Author)
	}
}

func grpcClientError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return books.ErrBookNotFound
	case codes.InvalidArgument:
		return books.InvalidBookIDError{}
	default:
		return err
	}
}

func printDomainError(id string, err error) {
	if errors.Is(err, books.ErrBookNotFound) {
		fmt.Printf("%q -> domain error: %s\n", id, books.ErrBookNotFound)
		return
	}
	var invalid books.InvalidBookIDError
	if errors.As(err, &invalid) {
		fmt.Printf("%q -> domain error: %s\n", id, invalid.Error())
		return
	}
	fmt.Printf("%q -> unexpected error: %s\n", id, err)
}
