# Quickstart gRPC

This example is a small gRPC service example, showcasing a unary `GreeterService`, a Tego facade
implementation, and a generated Tego facade client. It uses the shared generated quickstart package
in `../quickstart`, so the gRPC and Connect quickstarts can stay directly comparable without
duplicating the same proto schema.

Tego generates the `GreeterService` facade, an unimplemented embeddable service, a gRPC
adapter/server registration helper, and a facade client constructor.

Good files to start with:

- `../quickstart/proto/hello/v1/hello.proto`
- `../quickstart/hello/hello.tego.go`
- `cmd/server/main.go`
- `cmd/client/main.go`

Regenerate the shared quickstart package from `../quickstart`:

```sh
cd ../quickstart
buf generate --config ../../buf.dev.yaml --template buf.gen.yaml ../.. --path proto/hello/v1/hello.proto
```

Run the server and client in separate terminals:

```sh
go run ./cmd/server
go run ./cmd/client
```
