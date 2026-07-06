package main

import (
	"context"
	"fmt"
	"log"

	"github.com/seeruk/tego/examples/hooks/catalog"
	"github.com/seeruk/tego/examples/hooks/catalogpbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50056", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := catalog.NewCatalogServiceGRPCClient(catalogpbv1.NewCatalogServiceClient(conn))
	for _, id := range []string{"  TEGO  ", "", "missing"} {
		book, err := client.GetBook(context.Background(), id)
		if err != nil {
			fmt.Printf("%q -> %s\n", id, err)
			continue
		}
		fmt.Printf("%q -> %s [%s]\n", id, book.DisplayTitle, book.CatalogRef)
	}

	protoClient := catalogpbv1.NewCatalogServiceClient(conn)
	request := &catalogpbv1.GetBookRequest{}
	request.SetId("tego")
	response, err := protoClient.GetBook(context.Background(), request)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("raw protobuf legacy_book_id -> %s\n", response.GetBook().GetLegacyBookId())
}
