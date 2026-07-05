package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/error-mapping/books"
	"github.com/seeruk/tego/examples/error-mapping/bookspbv1/bookspbv1connect"
)

func main() {
	nativeClient := bookspbv1connect.NewBookServiceClient(http.DefaultClient, "http://localhost:8081")
	client := books.NewBookServiceConnectClient(
		nativeClient,
		tego.WithErrorMapper(connectClientError),
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

func connectClientError(err error) error {
	switch connect.CodeOf(err) {
	case connect.CodeNotFound:
		return books.ErrBookNotFound
	case connect.CodeInvalidArgument:
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
