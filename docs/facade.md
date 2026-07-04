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

Generated transport adapters should be fully implemented servers/handlers that call the facade:

```go
func NewTicketServiceGRPCServer(service TicketService) yirapbv1.TicketServiceServer
func NewTicketServiceConnectHandler(service TicketService, opts ...connect.HandlerOption) (string, http.Handler)
```

Those generated implementations should be embeddable, so users can override only the native
transport methods where they need lower-level access.

## Facade Options

Tego will introduce method options to customize facade signatures, mainly focused around inlining
request and response types to produce more Go-like interfaces:

```protobuf
option (tego.method).facade.inline_request = true;  // Request only
option (tego.method).facade.inline_response = true; // Response only
option (tego.method).facade.inline = true;          // Both request and response
```

These options are only suitable for unary methods. It may be desirable to be quite opinionated about
this, potentially even automatically having unary requests and responses inlined by default.

When inlining is enabled, Tego should generate `Pack` and `Unpack` value-shape helpers:

```go
PackGetTicketRequest(ticketID string) GetTicketRequest
UnpackGetTicketRequest(request GetTicketRequest) string
PackGetTicketResponse(ticket Ticket) GetTicketResponse
UnpackGetTicketResponse(response GetTicketResponse) Ticket
```

`Pack` and `Unpack` are pure value helpers. They are useful inside generated adapters and also
available to users who want to cross the facade/message boundary manually.

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
