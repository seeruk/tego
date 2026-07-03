package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/seeruk/tego/example/workflow"
	"github.com/seeruk/tego/example/yira"
	"github.com/seeruk/tego/example/yirapbv1/yirapbv1connect"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{
		Transport: &http.Transport{
			Protocols: protocols,
		},
	}

	nativeClient := yirapbv1connect.NewTicketServiceClient(httpClient, "http://localhost:8080")
	client := yira.NewTicketServiceConnectClient(nativeClient)

	if err := workflow.Run(ctx, "connect", client); err != nil {
		log.Fatal(err)
	}
}
