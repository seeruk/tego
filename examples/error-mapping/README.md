# Error Mapping

This example shows a facade service returning ordinary Go domain errors while the generated gRPC
and Connect adapters map those errors at the transport boundary.

The service returns a sentinel error for a missing book and a structured error for an invalid ID:

```go
var ErrBookNotFound = errors.New("book not found")

type InvalidBookIDError struct {
	ID string
}
```

In this example, the gRPC and Connect servers both use `tego.WithErrorMapper(...)`. Server-side 
mappers use `errors.Is` for the sentinel error, `errors.AsType` for the structured error, and fall 
back to Tego's default transport mapping:

```go
books.RegisterBookServiceGRPCServer(
	server,
	books.BookStore{},
	tego.WithErrorMapper(grpcError),
)
```

```go
path, handler := books.NewBookServiceConnectHandler(
	books.BookStore{},
	tego.WithErrorMapper(connectError),
)
```

The generated facade clients can use the same option in the other direction, mapping native
transport errors back to domain errors before callers see them:

```go
client := books.NewBookServiceGRPCClient(
	bookspbv1.NewBookServiceClient(conn),
	tego.WithErrorMapper(grpcClientError),
)
```

Good files to start with:

- `proto/books/v1/books.proto`
- `books/service.go`
- `cmd/grpc-server/main.go`
- `cmd/grpc-client/main.go`
- `cmd/connect-server/main.go`
- `cmd/connect-client/main.go`

Regenerate from this example folder:

```sh
buf generate
```

Run either server and client pair in separate terminals:

```sh
go run ./cmd/grpc-server
go run ./cmd/grpc-client
```

```sh
go run ./cmd/connect-server
go run ./cmd/connect-client
```
