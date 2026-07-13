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

A file that only provides reusable shapes can opt into Tego without inventing an output package:

```protobuf
option (tego.file).omit = true;

message Labels {
  option (tego.message).flatten = true;
  repeated string values = 1;
}
```

An omitted Tego file is still included in shape and type planning, but its effective plan must
produce no declarations. `omit: true` cannot be combined with `(tego.file).go_package` or
`(tego.file).output_path`.

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

The goal is boring Go; structs, enums, slices, maps, pointers. The protobuf code still exists, and 
Tego provides escape-hatches where necessary, but as much as possible the protobuf boundary stays 
explicit, leaving you with regular Go types that hopefully match your usual expectations.

By default, protobuf 64-bit integers generate as native-width Go `int` or `uint` fields. If a
message or field needs exact-width integers, set `preserve_integer_width`:

```protobuf
message Metrics {
  option (tego.message).fields.preserve_integer_width = true;

  int64 event_count = 1;
  uint64 byte_count = 2;
  int64 approximate_count = 3 [(tego.field).preserve_integer_width = false];
}
```

With the option enabled, `int64`, `sint64`, and `sfixed64` generate as `int64`, while `uint64` and
`fixed64` generate as `uint64`. Field-level values override the message-level default.

Tego does not unwrap protobuf value wrapper messages such as `google.protobuf.StringValue` into
scalar pointers. They are treated as ordinary protobuf message pointer types. You should use Tego's 
`nullable`, `omittable`, or nullable-shape modelling for presence and null semantics.

### Shapes

Protobuf doesn't support representing certain "shapes" of data that Go can. Tego adds support for 
some of these shapes, for example, slices of slices without an intermediary wrapper, maps with 
struct or enum keys (as long as they're comparable), or representing nullability as a pointer (in 
addition to omittability).

```protobuf
message ProjectSlug {
  // Flatten is often useful in combination with a `go_type` on the field in this type. 
  option (tego.message).flatten = true;

  string value = 1;
}

message TicketList {
  repeated Ticket tickets = 1;
}

message TicketsByPerson {
  message Map {
    Person key = 1;
    repeated Ticket value = 2;
  }
}

message Project {
  ProjectSlug slug = 1;
  repeated TicketList bucketed_tickets = 2;
  repeated TicketsByPerson tickets_by_author = 3;
}
```

```go
type Project struct {
	Slug string
	BucketedTickets [][]Ticket
	TicketsByAuthor map[Person][]Ticket
}
```

See [examples/shapes](examples/shapes) for a full example.

### Custom Types

When a generated field should really be one of your own types, give Tego the type and the conversion
functions:

```protobuf
int64 cost = 1 [(tego.field).go_type = {
  ref: "github.com/acme/project/money.Money"
  from_proto: "github.com/acme/project/money.MoneyFromProto"
  to_proto: "github.com/acme/project/money.MoneyToProto"
  comparable: true
}];
```

For Go types that are convertible to or from the protobuf field type, the corresponding conversion
function may be omitted and Tego will generate a Go cast instead:

```protobuf
int32 month = 1 [(tego.field).go_type = {
  ref: "time.Month"
}];
```

If supplied, `from_proto` or `to_proto` is always used for its direction, even when a direct Go
conversion is possible. This allows either side of the conversion to perform validation and return
errors while the other side uses an automatic conversion.

This works well in conjunction with the `(tego.message).flatten` option, but can be useful inline
for one-offs.

If your type is more complex, you can also specify the `go_type` option at the message level:

```protobuf
message Date {
  option (tego.message).go_type = {
    ref: "github.com/acme/project/date.Date"
    from_proto: "github.com/acme/project/date.DateFromProto"
    to_proto: "github.com/acme/project/date.DateToProto"
    comparable: true
  };
  
  int32 year = 1;
  int32 month = 2;
  int32 day = 3;
}
```

The `go_type` options accept predeclared Go value types, pointers, slices, fixed arrays, maps,
fully-qualified named types, and generic instantiations. These forms can be nested, so custom types
can use expressions such as `[12]uint`, `map[string][]uint64`, or
`github.com/acme/project/date.MonthlyArray[uint]`. Generic placeholders in `ref` can also be bound
to any supported expression through `type_args`. Those bindings are resolved recursively, so one
binding may reference another, but every chain must ultimately end in concrete predeclared or
fully-qualified named types. Missing bindings, cycles, and unused entries are rejected.

See [examples/custom-types](examples/custom-types) for a full example.

### Presence and Patches

For patch-style messages, Tego can generate `omittable.Value[T]` (from 
[seeruk/go-containers](https://github.com/seeruk/go-containers)) so callers can differentiate 
between not set, set to null/nil, and set to a value, as needed.

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

### Service Facades

Services generate a facade interface. Similar to models that Tego generates, the facade interface is
a plain Go interface with plain Go types. Tego generates an [adapter layer](#transport-adapters) for
gRPC and Connect.

Request and response messages are inlined by default when it is safe. This inlining can be disabled
on a per-method-basis. This keeps the facade interface looking like a regular Go service, not an RPC
interface. For example, a simple `SayHello` method:

```protobuf
service GreeterService {
  rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
}
```

Is inlined to something like this:

```go
type GreeterService interface {
	SayHello(context.Context, string) (string, error)
}
```

The facade is the interface I expect most Go code to implement and call. It keeps handlers from 
filling up with request/response wrapper plumbing.

One important note on streaming using facade clients; for server-streaming and bidi-streaming, 
methods return lazy `iter.Seq2` response streams. The native RPC is not opened until callers range 
the returned sequence. The sequence yields transport setup, receive, and mapping errors, so callers 
should always check the error side while ranging. It works this way to ensure resources don't leak 
and so that the interface doesn't require any wrapper types, like a `Stream[T]` with an explicit 
`Close` method. If you don't range the iterator, the underlying transport is never opened, and if 
you do range it, the transport is closed when iteration is complete.

### Transport Adapters

Whether you want gRPC, or Connect, Tego supports generating adapters and helpers to ease setting up
servers for both that use Tego's mapping. Tego features a few escape-hatches to give you more 
control over interacting with the native server either with overriding server methods, or via hooks
passed to the Tego adapters.

```go
hello.RegisterGreeterServiceGRPCServer(server, greeter{})

client := hello.NewGreeterServiceGRPCClient(
	hellopbv1.NewGreeterServiceClient(conn),
)
```

Your app should implement the facade. The generated gRPC or Connect server/client translates at the 
edge. If you need transport details, you can override a native method and delegate back to the 
generated adapter. See [examples/quickstart-grpc](examples/quickstart-grpc),
[examples/quickstart-connect](examples/quickstart-connect), and
[examples/transport-override](examples/transport-override).

### Adapter Hooks

Adapters can run hooks around request and response mapping, giving you a place for validation,
normalization, context setup, and response enrichment outside the service implementation. Generated
service hooks are method-specific and typed, while `tego.InterfaceHooks` with top-level helpers such
as `tego.AddPostRequestMappingHook` let you reuse hooks for values that implement an interface such
as `Validate() error`. See [examples/hooks](examples/hooks).

NOTE: The API for interface hooks _will_ change when Go 1.27 releases to take advantage of methods
being allowed to also have their own generic type parameters. I'll be reviewing the whole API of 
Tego to see what other opportunities for simplification may arise with the release of Go 1.27.

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
