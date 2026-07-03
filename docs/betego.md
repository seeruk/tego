# Betego: Better Tego Interfaces

Initial interfaces:

```go

// TicketServiceServer
type TicketServiceServer interface {
	ListTicketsRPC(context.Context, *tego.Request[ListTicketsRequest]) (*tego.Response[ListTicketsResponse], error)
	GetTicketRPC(context.Context, *tego.Request[GetTicketRequest]) (*tego.Response[GetTicketResponse], error)
	CreateTicketRPC(context.Context, *tego.Request[CreateTicketRequest]) (*tego.Response[CreateTicketResponse], error)
	UpdateTicketRPC(context.Context, *tego.Request[UpdateTicketRequest]) (*tego.Response[UpdateTicketResponse], error)
	CloseTicketRPC(context.Context, *tego.Request[CloseTicketRequest]) (*tego.Response[struct{}], error)
	WatchTicketEventsRPC(context.Context, *tego.Request[WatchTicketEventsRequest], *tego.ServerSendStream[TicketEvent]) error
	ImportTicketEventsRPC(context.Context, *tego.ServerRecvStream[TicketEvent]) (*tego.Response[ImportTicketEventsResponse], error)
	SyncTicketEventsRPC(context.Context, *tego.ServerBidiStream[TicketEvent, TicketEvent]) error
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
