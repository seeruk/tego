# Transport Override

This is a more advanced example, showcasing the escape hatch for native gRPC behaviour. The service
is implemented through the facade, but the server overrides one native gRPC method to inspect
metadata and set a response header before delegating to the generated adapter.

In this example, Tego generates a `ProfileServiceGRPCAdapter` with `AdaptGetProfile`, so custom
native methods can do transport work and then delegate to the facade adapter.

Good files to start with:

- `proto/override/v1/profile.proto`
- `override/profile.tego.go`
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
