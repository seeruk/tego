package main

import (
	"log"
	"net/http"

	"github.com/seeruk/tego/example/yira"
)

func main() {
	mux := http.NewServeMux()
	path, handler := yira.NewTicketServiceConnectHandler(yira.NewInMemoryTicketService())
	mux.Handle(path, handler)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:      "localhost:8080",
		Handler:   mux,
		Protocols: protocols,
	}

	log.Printf("serving Yira Connect example on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
