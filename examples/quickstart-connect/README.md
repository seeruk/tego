# Quickstart Connect

This mirrors the gRPC quickstart using Connect: the same unary `GreeterService`, facade
implementation, generated handler, and generated facade client. It uses the shared generated
quickstart package in `../quickstart`, so the gRPC and Connect quickstarts can stay directly
comparable without duplicating the same proto schema.

Tego generates the `GreeterService` facade, an unimplemented embeddable service, a Connect handler
helper, and a facade client constructor.

Good files to start with:

- `../quickstart/proto/hello/v1/hello.proto`
- `../quickstart/hello/hello.tego.go`
- `cmd/server/main.go`
- `cmd/client/main.go`

Regenerate the shared quickstart package from `../quickstart`:

```sh
cd ../quickstart
buf generate
```

Run the server and client in separate terminals:

```sh
go run ./cmd/server
go run ./cmd/client
```
