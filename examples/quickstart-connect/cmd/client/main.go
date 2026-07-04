package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/seeruk/tego/examples/quickstart/hello"
	"github.com/seeruk/tego/examples/quickstart/hellopbv1/hellopbv1connect"
)

func main() {
	nativeClient := hellopbv1connect.NewGreeterServiceClient(http.DefaultClient, "http://localhost:8080")
	client := hello.NewGreeterServiceConnectClient(nativeClient)

	message, err := client.SayHello(context.Background(), "Tego")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(message)
}
