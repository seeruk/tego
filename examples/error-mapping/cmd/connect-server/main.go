package main

import (
	"errors"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"github.com/seeruk/tego"
	"github.com/seeruk/tego/examples/error-mapping/books"
)

func main() {
	mux := http.NewServeMux()
	path, handler := books.NewBookServiceConnectHandler(
		books.BookStore{},
		tego.WithErrorMapper(connectError),
	)
	mux.Handle(path, handler)

	server := &http.Server{
		Addr:    "localhost:8081",
		Handler: mux,
	}

	log.Printf("serving Connect error-mapping example on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func connectError(err error) error {
	if errors.Is(err, books.ErrBookNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	if invalid, ok := errors.AsType[books.InvalidBookIDError](err); ok {
		return connect.NewError(connect.CodeInvalidArgument, invalid)
	}
	return tego.ConnectError(err)
}
