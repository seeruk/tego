# Tego Yira Example

This directory contains a runnable Yira issue-tracker example. The proto and generated protobuf/Tego
packages are shared by both transports, while the Connect and gRPC clients and servers live in
separate command packages.

Regenerate the example output from this directory:

```sh
buf generate .. --config ../buf.yaml --template buf.gen.yaml --path proto/yirapb/v1/yira.proto
```

Run the Connect server and client from the repository root:

```sh
go run ./example/connect/server
go run ./example/connect/client
```

The Connect server listens on `localhost:8080` and enables HTTP/1 plus unencrypted HTTP/2 through
`net/http.Protocols`.

Run the gRPC server and client from the repository root:

```sh
go run ./example/grpc/server
go run ./example/grpc/client
```

The gRPC server listens on `localhost:50051`.

Both clients use the generated Tego-native `yira.TicketServiceClient` interface and run the same
workflow: create, update, list, get, watch events, import events with client streaming, sync one
event with bidirectional streaming, and close a ticket. 
