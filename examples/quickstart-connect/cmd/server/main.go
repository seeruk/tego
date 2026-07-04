package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/seeruk/tego/examples/quickstart/hello"
)

type greeter struct {
	hello.UnimplementedGreeterService
}

func (greeter) SayHello(ctx context.Context, name string) (string, error) {
	return fmt.Sprintf("hello, %s", name), nil
}

func main() {
	mux := http.NewServeMux()
	path, handler := hello.NewGreeterServiceConnectHandler(greeter{})
	mux.Handle(path, handler)

	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: mux,
	}

	log.Printf("serving Connect quickstart on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
