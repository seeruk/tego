# Tego

A protoc plugin that emits a layer of idiomatic and modern Go over your protobuf and gRPC types.

> Tego /'te.ɡoː/ • Latin: To cover, shield, and protect.

## Installation and Usage

Install the generator binary somewhere Buf can find it:

```sh
go install github.com/seeruk/tego/cmd/protoc-gen-tego@latest
```

Add Tego's options module to your `buf.yaml`:

```yaml
version: v2

deps:
- buf.build/seeruk-oss/tego
```

Then opt a proto file into Tego output:

```protobuf
import "tego/options.proto";

option go_package = "github.com/acme/project/hellopbv1;hellopbv1";
option (tego.file).go_package = "github.com/acme/project/hello;hello";
```

And add Tego to `buf.gen.yaml` alongside the usual Go protobuf plugins:

```yaml
version: v2

plugins:
- local: protoc-gen-go
  out: .
  opt:
  - paths=import
  - module=github.com/acme/project
- local: protoc-gen-go-grpc
  out: .
  opt:
  - paths=import
  - module=github.com/acme/project
- local: protoc-gen-tego
  out: .
  opt:
  - paths=import
  - module=github.com/acme/project
  - rpc=grpc
```

Then run:

```sh
buf generate
```

Use `rpc=grpc` for gRPC, `rpc=connect` for Connect, `rpc=none` for just types and mapping
functions, or omit `rpc` to generate both gRPC and Connect adapters.

For Connect, swap `protoc-gen-go-grpc` for `protoc-gen-connect-go` and use `rpc=connect`. For
`rpc=none`, you only need `protoc-gen-go` and `protoc-gen-tego`.

If you are using `protoc` directly, you can still use Tego, but you need to make
`tego/options.proto` available on your proto include path.

## Core Concepts

Tego sits between protobuf's generated Go and the Go you probably want to write by hand. The
protobuf types still exist, but _your_ code can use smaller, plainer Go types and service
interfaces.

- [Generated Go package](#generated-go-package)
- [Types and mappings](#types-and-mappings)
- [Shapes](#shapes)
- [Custom types](#custom-types)
- [Presence and patches](#presence-and-patches)
- [Facade services](#facade-services)
- [Transport adapters](#transport-adapters)
- [Adapter hooks](#adapter-hooks)

### Generated Go Package

Every file that wants Tego output opts into a Go package for the friendly types:

```protobuf
option go_package = "github.com/acme/project/ticketpbv1;ticketpbv1";
option (tego.file).go_package = "github.com/acme/project/ticket;ticket";
```

The `go_package` option is still for protobuf's Go types. The `(tego.file).go_package` option is
where Tego writes the types you work with in the rest of your app.

### Types and Mappings

Tego generates plain Go structs and mapping functions beside the protobuf types:

```protobuf
message Ticket {
  string id = 1;
  string title = 2;
}
```

```go
type Ticket struct {
	ID    string
	Title string
}

func TicketFromProto(*ticketpbv1.Ticket) Ticket
func TicketToProto(Ticket) (*ticketpbv1.Ticket, error)
```

The goal is boring Go; structs, enums, slices, maps, pointers, with conversion code you can read.
The protobuf code still exists, and Tego provides escape-hatches where necessary, but as much as
possible the protobuf boundary stays explicit, leaving your with regular Go types that hopefully
match your usual expectations.

### Shapes

Some protobuf messages are only there to express a shape protobuf does not have directly. Tego can
collapse those when they are used as fields:

```protobuf
message TicketList {
  repeated Ticket tickets = 1;
}

message TicketSlug {
  option (tego.message).flatten = true;

  string value = 1;
}
```

```go
type Project struct {
	Tickets []Ticket
	Slug    string
}
```

This keeps the protobuf schema honest while keeping the Go model small. See
[examples/shapes](examples/shapes) for slices, maps, nullable values, and flattening.

### Custom Types

When a generated field should really be one of your own types, give Tego the type and the conversion
functions:

```protobuf
int64 credit_cents = 1 [(tego.field).go_type = {
  ref: "github.com/acme/project/money.Money"
  from_proto: "github.com/acme/project/money.MoneyFromProto"
  to_proto: "github.com/acme/project/money.MoneyToProto"
  comparable: true
}];
```

This is for keeping domain meaning in the Go code. An email can be an `Email`, money can be `Money`,
and IDs can be real types instead of strings with good intentions. See
[examples/custom-types](examples/custom-types).

### Presence and Patches

For patch-style messages, Tego can generate `omittable.Value[T]` so callers can differentiate between
not set, set to null/nil, and set to a value, as needed.

```protobuf
message UpdateProfileRequest {
  option (tego.message).fields.omittable = true;

  string display_name = 1;
  NullableString bio = 2;
  string actor_id = 3 [(tego.field).omittable = false];
}
```

```go
type UpdateProfileRequest struct {
	DisplayName omittable.Value[string]
	Bio         omittable.Value[*string]
	ActorID     string
}
```

See [examples/presence-patch](examples/presence-patch).

### Facade Services

Services generate a facade interface. Request and response messages are inlined by default when it
is safe. This inlining can be disabled on a per-method-basis.

```protobuf
service GreeterService {
  rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
}
```

```go
type GreeterService interface {
	SayHello(context.Context, string) (string, error)
}
```

This is the interface I expect most Go code to implement and call. It keeps handlers from filling up
with request/response wrapper plumbing.

### Transport Adapters

If you still want gRPC or Connect, Tego can generate the adapter layer too:

```go
hello.RegisterGreeterServiceGRPCServer(server, greeter{})

client := hello.NewGreeterServiceGRPCClient(
	hellopbv1.NewGreeterServiceClient(conn),
)
```

Your app implements the facade. The generated gRPC or Connect server/client translates at the edge.
If you need transport details, you can override a native method and delegate back to the generated
adapter. See [examples/quickstart-grpc](examples/quickstart-grpc),
[examples/quickstart-connect](examples/quickstart-connect), and
[examples/transport-override](examples/transport-override).

### Adapter Hooks

Adapters can run hooks around request and response mapping, giving you a place for validation,
normalization, context setup, and response enrichment outside the service implementation. Generated
service hooks are method-specific and typed, while `tego.InterfaceHooks` let you reuse hooks for
values that implement an interface such as `Validate() error`. See [examples/hooks](examples/hooks).

### Error Mapping

Facade implementations can return ordinary Go errors from your domain. At the transport boundary,
pass `tego.WithErrorMapper(...)` to map those errors to native gRPC or Connect errors:

```go
books.RegisterBookServiceGRPCServer(
	server,
	books.BookStore{},
	tego.WithErrorMapper(grpcError),
)
```

For Connect, native handler options are wrapped with `tego.WithConnectHandlerOptions(...)` so they
can share the same generated constructor:

```go
path, handler := books.NewBookServiceConnectHandler(
	books.BookStore{},
	tego.WithErrorMapper(connectError),
	tego.WithConnectHandlerOptions(connect.WithInterceptors(auth)),
)
```

See [examples/error-mapping](examples/error-mapping) for sentinel errors with `errors.Is` and
structured errors with `errors.AsType`. Generated facade clients can use the same option to map 
native transport errors back to domain errors before returning them to your code.

## Examples

The [examples suite](examples/README.md) is the best place to start. It has small, focused examples
for gRPC, Connect, error mapping, adapter hooks, generated shapes, options, custom types, patch
semantics, streaming, transport overrides, and a kitchen-sink type reference.

Tego is intentionally focused on generated Go types, mapping code, facade service interfaces, and
optional gRPC/Connect adapters. It does not try to own your transport, application framework,
storage, validation, or domain model boundaries.

## License

MIT

## TODO

* Extended handling of `Any` WKT
