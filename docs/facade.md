# Facade: "Native" Go Interfaces

Tego is all about generating nice Go types, so to that end, it should probably also support 
generating pleasant and idiomatic Go service interfaces. In other words, a simple facade over the
"real" Tego interface, or the "full" version.

Tego can generate layers of adapter code to enable multiple approaches for using Tego. Some users 
may want to just use the Tego interface directly. Some may want a more Go-native approach. Some may
want both! We can support all of these options.

To do so, we'll make use of several separate interfaces, with intentional names.

```go
// TicketServiceServer
type TicketServiceServer interface {
	ListTicketsHandler(context.Context, *tego.Request[ListTicketsRequest]) (*tego.Response[ListTicketsResponse], error)
	GetTicketHandler(context.Context, *tego.Request[GetTicketRequest]) (*tego.Response[GetTicketResponse], error)
	CreateTicketHandler(context.Context, *tego.Request[CreateTicketRequest]) (*tego.Response[CreateTicketResponse], error)
	UpdateTicketHandler(context.Context, *tego.Request[UpdateTicketRequest]) (*tego.Response[UpdateTicketResponse], error)
	CloseTicketHandler(context.Context, *tego.Request[CloseTicketRequest]) (*tego.Response[struct{}], error)
	WatchTicketEventsHandler(context.Context, *tego.Request[WatchTicketEventsRequest], *tego.ServerSendStream[TicketEvent]) error
	ImportTicketEventsHandler(context.Context, *tego.ServerRecvStream[TicketEvent]) (*tego.Response[ImportTicketEventsResponse], error)
	SyncTicketEventsHandler(context.Context, *tego.ServerBidiStream[TicketEvent, TicketEvent]) error
}

// TicketServiceClient is the client interface for TicketService.
type TicketServiceClient interface {
	ListTickets(context.Context, *tego.Request[ListTicketsRequest]) (*tego.Response[ListTicketsResponse], error)
	GetTicket(context.Context, *tego.Request[GetTicketRequest]) (*tego.Response[GetTicketResponse], error)
	CreateTicket(context.Context, *tego.Request[CreateTicketRequest]) (*tego.Response[CreateTicketResponse], error)
	UpdateTicket(context.Context, *tego.Request[UpdateTicketRequest]) (*tego.Response[UpdateTicketResponse], error)
	CloseTicket(context.Context, *tego.Request[CloseTicketRequest]) (*tego.Response[struct{}], error)
	WatchTicketEvents(context.Context, *tego.Request[WatchTicketEventsRequest]) (*tego.ClientRecvStream[TicketEvent], error)
	ImportTicketEvents(context.Context, ...tego.CallOption) (*tego.ClientSendStream[TicketEvent, ImportTicketEventsResponse], error)
	SyncTicketEvents(context.Context, ...tego.CallOption) (*tego.ClientBidiStream[TicketEvent, TicketEvent], error)

	AsTicketService() TicketService
}

// TicketService...
type TicketService interface {
	ListTickets(context.Context, ListTicketsRequest) (ListTicketsResponse, error)
	GetTicket(context.Context, GetTicketRequest) (GetTicketResponse, error)
	CreateTicket(context.Context, CreateTicketRequest) (CreateTicketResponse, error)
	UpdateTicket(context.Context, UpdateTicketRequest) (UpdateTicketResponse, error)
	CloseTicket(context.Context, CloseTicketRequest) error
	WatchTicketEvents(context.Context, WatchTicketEventsRequest) (iter.Seq2[TicketEvent, error], error)
	ImportTicketEvents(context.Context, iter.Seq[TicketEvent]) (ImportTicketEventsResponse, error)
	SyncTicketEvents(context.Context, iter.Seq[TicketEvent]) (iter.Seq2[TicketEvent, error], error)
}

// tego.EmptyResponse => tego.Response[struct{}]
```

For responses which are "empty", as in `struct{}`, we should omit them from the plain Go type.

Tego will introduce new options to customize this interface, mainly focused around inlining request
and response types to produce more "Go-like" interfaces. These options will be:

```protobuf
option (tego.method).facade.inline_request = true;  // Request only
option (tego.method).facade.inline_response = true; // Response only
option (tego.method).facade.inline = true;          // Both request and response
```
