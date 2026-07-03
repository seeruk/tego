package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/example/yira"
	"github.com/seeruk/tego/omittable"

	types "github.com/seeruk/tego/example/yira/types"
)

func Run(ctx context.Context, transport string, client yira.TicketServiceClient) error {
	actor := yira.Person{
		ID:          "user-example-client",
		DisplayName: "Example Client",
		Email:       "client@yira.example",
	}

	draft := yira.TicketDraft{
		ProjectID:   yira.DefaultProjectID,
		Title:       fmt.Sprintf("%s adapter follow-up", transport),
		Description: "Exercise the generated Tego client API end to end.",
		Priority:    yira.TicketPriorityHigh,
		Reporter:    actor,
		DueDate:     types.Date(time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)),
		Labels: types.Set[types.Label]{
			types.Label("example"):   {},
			types.Label(transport):   {},
			types.Label("generated"): {},
		},
		Metadata: tego.Struct{
			"transport": transport,
			"source":    "example workflow",
		},
	}

	create := tego.NewRequest(yira.CreateTicketRequest{Ticket: draft})
	create.Header().Set("x-yira-client", transport)
	createResponse, err := client.CreateTicket(ctx, create)
	if err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	ticket := createResponse.Message.Ticket
	fmt.Printf("[%s] created %s: %s\n", transport, ticket.ID, ticket.Title)

	patch := yira.TicketPatch{
		Status:  omittable.Some(yira.TicketStatusInProgress),
		Version: "example-workflow",
	}
	updateResponse, err := client.UpdateTicket(ctx, tego.NewRequest(yira.UpdateTicketRequest{
		TicketID: ticket.ID,
		Patch:    patch,
	}))
	if err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	ticket = updateResponse.Message.Ticket
	fmt.Printf("[%s] updated %s to status %d\n", transport, ticket.ID, ticket.Status)

	listResponse, err := client.ListTickets(ctx, tego.NewRequest(yira.ListTicketsRequest{
		ProjectID: yira.DefaultProjectID,
		Cursor:    yira.CursorRequest{Limit: 5},
	}))
	if err != nil {
		return fmt.Errorf("list tickets: %w", err)
	}
	fmt.Printf("[%s] listed %d tickets\n", transport, len(listResponse.Message.Tickets))

	getResponse, err := client.GetTicket(ctx, tego.NewRequest(yira.GetTicketRequest{TicketID: ticket.ID}))
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	fmt.Printf("[%s] fetched %s with %d event(s)\n", transport, getResponse.Message.Ticket.ID, len(getResponse.Message.Ticket.Events))

	watch, err := client.WatchTicketEvents(ctx, tego.NewRequest(yira.WatchTicketEventsRequest{
		ProjectID: yira.DefaultProjectID,
		TicketID:  ticket.ID,
	}))
	if err != nil {
		return fmt.Errorf("watch ticket events: %w", err)
	}
	watched, err := receiveAll(watch)
	if err != nil {
		return fmt.Errorf("receive watched events: %w", err)
	}
	fmt.Printf("[%s] watched %d event(s)\n", transport, watched)

	imported, err := importEvents(ctx, transport, client, actor)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] imported %d event(s)\n", transport, imported)

	if err := syncEvent(ctx, transport, client, actor); err != nil {
		return err
	}

	_, err = client.CloseTicket(ctx, tego.NewRequest(yira.CloseTicketRequest{
		TicketID:   ticket.ID,
		Resolution: "Verified through the runnable example.",
	}))
	if err != nil {
		return fmt.Errorf("close ticket: %w", err)
	}
	fmt.Printf("[%s] closed %s\n", transport, ticket.ID)

	return nil
}

func receiveAll(stream *tego.ClientRecvStream[yira.TicketEvent]) (int, error) {
	defer func() {
		_ = stream.Close()
	}()

	var count int
	for {
		event, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return count, nil
			}
			return count, err
		}
		count++
		fmt.Printf("  event %s: %s\n", event.ID, event.Note)
	}
}

func importEvents(
	ctx context.Context,
	transport string,
	client yira.TicketServiceClient,
	actor yira.Person,
) (int32, error) {
	stream, err := client.ImportTicketEvents(
		ctx,
		tego.WithCallHeader(tego.Metadata{"x-yira-client": []string{transport}}),
	)
	if err != nil {
		return 0, fmt.Errorf("open import stream: %w", err)
	}

	for i := range 2 {
		err := stream.Send(yira.TicketEvent{
			Kind:        yira.TicketEventKindCommented,
			Actor:       actor,
			Note:        fmt.Sprintf("Imported %s event %d", transport, i+1),
			Payload:     map[string]any{"transport": transport, "imported": true},
			Attachments: tego.ListValue{fmt.Sprintf("%s-import-%d", transport, i+1)},
		})
		if err != nil {
			return 0, fmt.Errorf("send import event: %w", err)
		}
	}

	response, err := stream.CloseAndReceive()
	if err != nil {
		return 0, fmt.Errorf("close import stream: %w", err)
	}
	return response.Message.ImportedCount, nil
}

func syncEvent(
	ctx context.Context,
	transport string,
	client yira.TicketServiceClient,
	actor yira.Person,
) error {
	stream, err := client.SyncTicketEvents(
		ctx,
		tego.WithCallHeader(tego.Metadata{"x-yira-client": []string{transport}}),
	)
	if err != nil {
		return fmt.Errorf("open sync stream: %w", err)
	}

	type syncResult struct {
		event yira.TicketEvent
		err   error
	}
	received := make(chan syncResult, 1)
	go func() {
		event, err := stream.Receive()
		received <- syncResult{event: event, err: err}
	}()

	err = stream.Send(yira.TicketEvent{
		Kind:    yira.TicketEventKindUpdated,
		Actor:   actor,
		Note:    fmt.Sprintf("Synced over %s", transport),
		Payload: map[string]any{"transport": transport, "synced": true},
	})
	if err != nil {
		return fmt.Errorf("send sync event: %w", err)
	}
	if err := stream.CloseRequest(); err != nil {
		return fmt.Errorf("close sync request: %w", err)
	}

	result := <-received
	if result.err != nil {
		return fmt.Errorf("receive sync event: %w", result.err)
	}
	fmt.Printf("[%s] synced event %s\n", transport, result.event.ID)

	if err := stream.CloseResponse(); err != nil && !errors.Is(err, tego.ErrUnsupported) {
		return fmt.Errorf("close sync response: %w", err)
	}
	return nil
}
