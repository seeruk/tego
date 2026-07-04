# Quickstart gRPC

This example is a small gRPC service example, showcasing a unary `GreeterService`, a Tego facade
implementation, and a generated Tego facade client.

Tego generates the `GreeterService` facade, an unimplemented embeddable service, a gRPC
adapter/server registration helper, and a facade client constructor.

Good files to start with:

- `proto/hello/v1/greeter.proto`
- `hello/greeter.tego.go`
- `cmd/server/main.go`
- `cmd/client/main.go`

From this example folder, regenerate with:

```sh
buf generate
```

Run the server and client in separate terminals:

```sh
go run ./cmd/server
go run ./cmd/client
```
