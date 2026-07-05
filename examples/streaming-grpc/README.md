# Streaming gRPC

This example shows server streaming, client streaming, and bidi streaming through Tego's facade
iterator signatures.

Tego generates facade methods that use `iter.Seq2[T, error]` for stream inputs and outputs, plus the
gRPC adapter and facade client.

Good files to start with:

- `proto/streaming/v1/events.proto`
- `streaming/events.tego.go`
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
