# Facade: Go Service Interfaces

Tego is all about generating nice Go types, so service generation should center on pleasant,
idiomatic Go service interfaces too. The facade is the primary Go-facing service API: it hides
protobuf messages, transport request/response wrappers, and framework-specific stream types behind
ordinary Go methods and Tego-generated value mappings.

The current direction is facade-first. Tego should generate idiomatic Go types, a facade service
interface, and transport-specific gRPC and/or Connect adapters that call that facade. The previous
idea of a public, transport-neutral Tego RPC service layer is no longer expected to be the default
surface. If a user needs an escape hatch for metadata, trailers, native streams, or exact
transport behaviour, they can override the generated gRPC or Connect method for the transport they
actually use.

## Interfaces

The facade service should use the clean service name:

```go
type TicketService interface {
	ListTickets(context.Context, ListTicketsRequest) (ListTicketsResponse, error)
	GetTicket(context.Context, GetTicketRequest) (GetTicketResponse, error)
	CreateTicket(context.Context, CreateTicketRequest) (CreateTicketResponse, error)
	UpdateTicket(context.Context, UpdateTicketRequest) (UpdateTicketResponse, error)
	CloseTicket(context.Context, CloseTicketRequest) error
	WatchTicketEvents(context.Context, WatchTicketEventsRequest) (iter.Seq2[TicketEvent, error], error)
	ImportTicketEvents(context.Context, iter.Seq2[TicketEvent, error]) (ImportTicketEventsResponse, error)
	SyncTicketEvents(context.Context, iter.Seq2[TicketEvent, error]) (iter.Seq2[TicketEvent, error], error)
}
```

For responses which are "empty", as in `struct{}`, omit them from the facade method and return
only `error`.

Tego should also generate an embeddable unimplemented facade implementation:

```go
type UnimplementedTicketService struct{}

func (UnimplementedTicketService) GetTicket(context.Context, GetTicketRequest) (GetTicketResponse, error) {
	var zero GetTicketResponse
	return zero, unimplementedTicketServiceError("GetTicket")
}
```

The helper wraps `tego.ErrUnimplemented`, allowing users and transport adapters to use
`errors.Is(err, tego.ErrUnimplemented)`.

Transport-specific error conversion should live in Tego rather than being generated once per
service. Generated adapters call `tego.GRPCError(err)` and `tego.ConnectError(err)` for facade
call errors and iterator-yielded errors, mapping `tego.ErrUnimplemented` to the native
unimplemented code while passing other errors through unchanged.

Generated transport adapters should be fully implemented servers/handlers that call the facade:

```go
func NewTicketServiceGRPCServer(service TicketService) yirapbv1.TicketServiceServer
func NewTicketServiceConnectHandler(service TicketService, opts ...connect.HandlerOption) (string, http.Handler)
```

Those generated implementations should be embeddable, so users can override only the native
transport methods where they need lower-level access.

## Facade Options

Tego supports service and method options to customize facade signatures, mainly focused around
inlining request and response types to produce more Go-like interfaces. Services inline safely
inlineable method sides by default, even when the service option is omitted:

```protobuf
option (tego.service).inline_by_default = true;
```

Methods can override the service default. `inline` sets both sides, and side-specific options win
afterwards:

```protobuf
option (tego.method).inline_request = true;  // Request only
option (tego.method).inline_response = true; // Response only
option (tego.method).inline = true;          // Both request and response
option (tego.method).inline = false;         // Neither side
```

Inlining applies to unary request and response sides, server-streaming request sides, and
client-streaming response sides. Bidi streaming methods are not inlined. Empty responses keep the
existing `error`-only facade shape.

When inlining is enabled, Tego generates `ToInline` and `FromInline` facade call-shape helpers.
Request helpers carry `context.Context` through so they can be passed directly to facade methods:

```go
GetTicketRequestToInline(ctx context.Context, request GetTicketRequest) (context.Context, string)
GetTicketRequestFromInline(ctx context.Context, ticketID string) (context.Context, GetTicketRequest)
```

For messages with multiple fields, request helpers return `context.Context` followed by one result
per inlined field:

```go
ListTicketsRequestToInline(
	ctx context.Context,
	request ListTicketsRequest,
) (
	context.Context,
	string,
	TicketFilter,
	CursorRequest,
)
ListTicketsRequestFromInline(
	ctx context.Context,
	projectID string,
	filter TicketFilter,
	cursor CursorRequest,
) (context.Context, ListTicketsRequest)
```

Response helpers accept and return `error`, so the result of a facade call can be passed directly
into the helper:

```go
GetTicketResponseToInline(response GetTicketResponse, err error) (Ticket, error)
GetTicketResponseFromInline(ticket Ticket, err error) (GetTicketResponse, error)
```

If the incoming error is non-nil, response helpers return the zero value for the response shape and
pass the error through unchanged.

With these helpers, generated adapters and user-owned transport overrides can forward through the
facade concisely:

```go
response, err := GetTicketResponseFromInline(
	a.service.GetTicket(GetTicketRequestToInline(ctx, request)),
)
```

For multi-field request or response inlining, the same direct call shape should work because the
helpers absorb the surrounding `context.Context` and `error` values.

## Transport Adapters

Generate transport-specific adapter structs to keep method helper names short and give the
adaptation code a clear home:

```go
type TicketServiceGRPCAdapter struct {
	service TicketService
}

func NewTicketServiceGRPCAdapter(service TicketService) *TicketServiceGRPCAdapter

func (a *TicketServiceGRPCAdapter) AdaptGetTicket(
	ctx context.Context,
	request *yirapbv1.GetTicketRequest,
) (*yirapbv1.GetTicketResponse, error)
```

```go
type TicketServiceConnectAdapter struct {
	service TicketService
}

func NewTicketServiceConnectAdapter(service TicketService) *TicketServiceConnectAdapter

func (a *TicketServiceConnectAdapter) AdaptGetTicket(
	ctx context.Context,
	request *connect.Request[yirapbv1.GetTicketRequest],
) (*connect.Response[yirapbv1.GetTicketResponse], error)
```

Generated gRPC servers and Connect handlers should use these adapter methods internally. Users can
also keep an adapter and call `AdaptGetTicket` from their own native overrides after inspecting or
modifying transport-specific state.

Example gRPC shape:

```go
type ticketServiceGRPCServer struct {
	yirapbv1.UnimplementedTicketServiceServer
	*TicketServiceGRPCAdapter
}

func NewTicketServiceGRPCServer(service TicketService) yirapbv1.TicketServiceServer {
	return &ticketServiceGRPCServer{
		TicketServiceGRPCAdapter: NewTicketServiceGRPCAdapter(service),
	}
}

func (s *ticketServiceGRPCServer) GetTicket(
	ctx context.Context,
	request *yirapbv1.GetTicketRequest,
) (*yirapbv1.GetTicketResponse, error) {
	return s.AdaptGetTicket(ctx, request)
}
```

## Streaming

Inbound streams should use `iter.Seq2[T, error]` rather than `iter.Seq[T]`, because native
gRPC/Connect stream receive operations are value-or-error shaped. This keeps receive and mapping
errors visible to facade implementations instead of hiding them in the adapter.

For server-streaming methods, the adapter calls the facade method, ranges the returned iterator,
returns iterator errors, and sends each yielded value to the native transport stream. For
client-streaming methods, the adapter builds an inbound `Seq2` from the native receive operation,
passes it to the facade method, and packs the returned response into the native transport response.

For bidirectional-streaming methods, the adapter should stay lazy: it builds an inbound `Seq2` from
the native receive operation, calls the facade method, and ranges the outbound `Seq2` to send native
responses. The facade implementation owns the stream choreography. A simple implementation can range
inbound inside the outbound iterator for lockstep request/response behaviour. A more complex
implementation can use goroutines and channels internally when it needs fuller duplex behaviour.
Native gRPC or Connect overrides remain the escape hatch for exact send/receive ordering, metadata,
half-close timing, or native stream access.
