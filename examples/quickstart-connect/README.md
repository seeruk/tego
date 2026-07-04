# Quickstart Connect

This mirrors the gRPC quickstart with Connect-only generation: the same unary `GreeterService`,
facade implementation, generated handler, and generated facade client.

Tego generates the `GreeterService` facade, an unimplemented embeddable service, a Connect handler
helper, and a facade client constructor.

Good files to start with:

- `proto/hello/v1/hello.proto`
- `hello/hello.tego.go`
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
