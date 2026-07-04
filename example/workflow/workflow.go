package workflow

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/seeruk/tego"
	"github.com/seeruk/tego/example/yira"
	"github.com/seeruk/tego/omittable"

	types "github.com/seeruk/tego/example/yira/types"
)

func Run(ctx context.Context, transport string, client yira.TicketService) error {
	actor := yira.Person{
		ID:          "user-example-client",
		DisplayName: "Example Client",
		Email:       "client@yira.example",
	}

	draft := yira.TicketDraft{
		ProjectID:   yira.DefaultProjectID,
		Title:       fmt.Sprintf("%s adapter follow-up", transport),
		Description: "Exercise the generated facade client API end to end.",
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

	createResponse, err := client.CreateTicket(ctx, yira.CreateTicketRequest{Ticket: draft})
	if err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	ticket := createResponse.Ticket
	fmt.Printf("[%s] created %s: %s\n", transport, ticket.ID, ticket.Title)

	patch := yira.TicketPatch{
		Status:  omittable.Some(yira.TicketStatusInProgress),
		Version: "example-workflow",
	}
	updateResponse, err := client.UpdateTicket(ctx, yira.UpdateTicketRequest{
		TicketID: ticket.ID,
		Patch:    patch,
	})
	if err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	ticket = updateResponse.Ticket
	fmt.Printf("[%s] updated %s to status %d\n", transport, ticket.ID, ticket.Status)

	listResponse, err := client.ListTickets(ctx, yira.ListTicketsRequest{
		ProjectID: yira.DefaultProjectID,
		Cursor:    yira.CursorRequest{Limit: 5},
	})
	if err != nil {
		return fmt.Errorf("list tickets: %w", err)
	}
	fmt.Printf("[%s] listed %d tickets\n", transport, len(listResponse.Tickets))

	getResponse, err := client.GetTicket(ctx, yira.GetTicketRequest{TicketID: ticket.ID})
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	fmt.Printf("[%s] fetched %s with %d event(s)\n", transport, getResponse.Ticket.ID, len(getResponse.Ticket.Events))

	watch, err := client.WatchTicketEvents(ctx, yira.WatchTicketEventsRequest{
		ProjectID: yira.DefaultProjectID,
		TicketID:  ticket.ID,
	})
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

	if err := client.CloseTicket(ctx, yira.CloseTicketRequest{
		TicketID:   ticket.ID,
		Resolution: "Verified through the runnable example.",
	}); err != nil {
		return fmt.Errorf("close ticket: %w", err)
	}
	fmt.Printf("[%s] closed %s\n", transport, ticket.ID)

	return nil
}

func receiveAll(events iter.Seq2[yira.TicketEvent, error]) (int, error) {
	var count int
	for event, err := range events {
		if err != nil {
			return count, err
		}
		count++
		fmt.Printf("  event %s: %s\n", event.ID, event.Note)
	}
	return count, nil
}

func importEvents(
	ctx context.Context,
	transport string,
	client yira.TicketService,
	actor yira.Person,
) (int32, error) {
	events := func(yield func(yira.TicketEvent, error) bool) {
		for i := range 2 {
			if !yield(yira.TicketEvent{
				Kind:        yira.TicketEventKindCommented,
				Actor:       actor,
				Note:        fmt.Sprintf("Imported %s event %d", transport, i+1),
				Payload:     map[string]any{"transport": transport, "imported": true},
				Attachments: tego.ListValue{fmt.Sprintf("%s-import-%d", transport, i+1)},
			}, nil) {
				return
			}
		}
	}

	response, err := client.ImportTicketEvents(ctx, events)
	if err != nil {
		return 0, fmt.Errorf("import events: %w", err)
	}
	return response.ImportedCount, nil
}

func syncEvent(
	ctx context.Context,
	transport string,
	client yira.TicketService,
	actor yira.Person,
) error {
	requests := func(yield func(yira.TicketEvent, error) bool) {
		yield(yira.TicketEvent{
			Kind:    yira.TicketEventKindUpdated,
			Actor:   actor,
			Note:    fmt.Sprintf("Synced over %s", transport),
			Payload: map[string]any{"transport": transport, "synced": true},
		}, nil)
	}

	responses, err := client.SyncTicketEvents(ctx, requests)
	if err != nil {
		return fmt.Errorf("sync events: %w", err)
	}

	for event, err := range responses {
		if err != nil {
			return fmt.Errorf("receive sync event: %w", err)
		}
		fmt.Printf("[%s] synced event %s\n", transport, event.ID)
		return nil
	}
	return fmt.Errorf("receive sync event: no response")
}
