package yira

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/seeruk/tego"

	types "github.com/seeruk/tego/example/yira/types"
)

// DefaultProjectID is the seeded project used by the runnable examples.
const DefaultProjectID = "yira-example"

// InMemoryTicketService is a small mutex-backed TicketService implementation for examples.
type InMemoryTicketService struct {
	mu         sync.RWMutex
	tickets    map[string]Ticket
	order      []string
	events     []TicketEvent
	nextTicket int
	nextEvent  int
	system     Person
}

var _ TicketService = (*InMemoryTicketService)(nil)

// NewInMemoryTicketService creates a deterministic in-memory Yira service.
func NewInMemoryTicketService() *InMemoryTicketService {
	service := &InMemoryTicketService{
		tickets: make(map[string]Ticket),
		system: Person{
			ID:          "user-system",
			DisplayName: "Yira System",
			Email:       "system@yira.example",
		},
	}
	service.seed(gofakeit.New(20260703))
	return service
}

func (s *InMemoryTicketService) ListTickets(
	_ context.Context,
	request *tego.Request[ListTicketsRequest],
) (*tego.Response[ListTicketsResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	projectID := request.Message.ProjectID
	if projectID == "" {
		projectID = DefaultProjectID
	}

	filtered := make([]Ticket, 0, len(s.tickets))
	for _, id := range s.order {
		ticket := s.tickets[id]
		if ticket.ProjectID != projectID || !matchesFilter(ticket, request.Message.Filter) {
			continue
		}
		filtered = append(filtered, cloneTicket(ticket))
	}

	start := 0
	if after := request.Message.Cursor.AfterCursor; after != "" {
		for i, ticket := range filtered {
			if ticket.ID == after {
				start = i + 1
				break
			}
		}
	}

	limit := int(request.Message.Cursor.Limit)
	if limit <= 0 || limit > 20 {
		limit = 20
	}

	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	page := filtered[start:end]
	var nextCursor string
	if end < len(filtered) {
		nextCursor = filtered[end-1].ID
	}

	return tego.NewResponse(ListTicketsResponse{
		Tickets:         page,
		TicketsByStatus: ticketsByStatus(page),
		Cursor: CursorResponse{
			NextCursor: nextCursor,
		},
	}), nil
}

func (s *InMemoryTicketService) GetTicket(
	_ context.Context,
	request *tego.Request[GetTicketRequest],
) (*tego.Response[GetTicketResponse], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ticket, ok := s.tickets[request.Message.TicketID]
	if !ok {
		return nil, fmt.Errorf("ticket %q not found", request.Message.TicketID)
	}
	return tego.NewResponse(GetTicketResponse{Ticket: cloneTicket(ticket)}), nil
}

func (s *InMemoryTicketService) CreateTicket(
	_ context.Context,
	request *tego.Request[CreateTicketRequest],
) (*tego.Response[CreateTicketResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket := s.createTicketLocked(request.Message.Ticket)
	return tego.NewResponse(CreateTicketResponse{Ticket: cloneTicket(ticket)}), nil
}

func (s *InMemoryTicketService) UpdateTicket(
	_ context.Context,
	request *tego.Request[UpdateTicketRequest],
) (*tego.Response[UpdateTicketResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[request.Message.TicketID]
	if !ok {
		return nil, fmt.Errorf("ticket %q not found", request.Message.TicketID)
	}

	patch := request.Message.Patch
	applyPatch(&ticket, patch)
	s.addEventLocked(&ticket, TicketEvent{
		Kind:    TicketEventKindUpdated,
		Actor:   s.system,
		Note:    "Ticket updated",
		Payload: tego.Value(map[string]any{"version": patch.Version}),
	})
	s.tickets[ticket.ID] = ticket

	return tego.NewResponse(UpdateTicketResponse{Ticket: cloneTicket(ticket)}), nil
}

func (s *InMemoryTicketService) CloseTicket(
	_ context.Context,
	request *tego.Request[CloseTicketRequest],
) (*tego.Response[struct{}], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[request.Message.TicketID]
	if !ok {
		return nil, fmt.Errorf("ticket %q not found", request.Message.TicketID)
	}

	ticket.Status = TicketStatusClosed
	s.addEventLocked(&ticket, TicketEvent{
		Kind:    TicketEventKindClosed,
		Actor:   s.system,
		Note:    request.Message.Resolution,
		Payload: tego.Value(map[string]any{"resolution": request.Message.Resolution}),
	})
	s.tickets[ticket.ID] = ticket

	return tego.NewResponse(struct{}{}), nil
}

func (s *InMemoryTicketService) WatchTicketEvents(
	_ context.Context,
	request *tego.Request[WatchTicketEventsRequest],
	stream *tego.ServerSendStream[TicketEvent],
) error {
	s.mu.RLock()
	events := s.eventsFor(request.Message.ProjectID, request.Message.TicketID)
	s.mu.RUnlock()

	for _, event := range events {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	return nil
}

func (s *InMemoryTicketService) ImportTicketEvents(
	_ context.Context,
	stream *tego.ServerRecvStream[TicketEvent],
) (*tego.Response[ImportTicketEventsResponse], error) {
	var imported int32
	for {
		event, err := stream.Receive()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		s.mu.Lock()
		s.addLooseEventLocked(event)
		s.mu.Unlock()
		imported++
	}

	return tego.NewResponse(ImportTicketEventsResponse{ImportedCount: imported}), nil
}

func (s *InMemoryTicketService) SyncTicketEvents(
	_ context.Context,
	stream *tego.ServerBidiStream[TicketEvent, TicketEvent],
) error {
	for {
		event, err := stream.Receive()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		s.mu.Lock()
		event = s.addLooseEventLocked(event)
		s.mu.Unlock()

		if err := stream.Send(event); err != nil {
			return err
		}
	}
}

func (s *InMemoryTicketService) seed(fake *gofakeit.Faker) {
	people := make([]Person, 0, 5)
	for i := range 5 {
		people = append(people, Person{
			ID:          fmt.Sprintf("user-%03d", i+1),
			DisplayName: fake.Name(),
			Email:       fake.Email(),
		})
	}

	statuses := []TicketStatus{
		TicketStatusOpen,
		TicketStatusInProgress,
		TicketStatusBlocked,
		TicketStatusOpen,
	}
	priorities := []TicketPriority{
		TicketPriorityNormal,
		TicketPriorityHigh,
		TicketPriorityLow,
		TicketPriorityNormal,
	}
	labels := []types.Label{"bug", "frontend", "backend", "billing", "support", "docs"}

	for i := range 4 {
		reporter := people[i%len(people)]
		draft := TicketDraft{
			ProjectID:   DefaultProjectID,
			Title:       fake.ProductName(),
			Description: fake.HackerPhrase(),
			Priority:    priorities[i%len(priorities)],
			Reporter:    reporter,
			DueDate: types.Date(fake.DateRange(
				time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC),
			)),
			Labels: labelSet(
				labels[i%len(labels)],
				labels[(i+2)%len(labels)],
			),
			Metadata: tego.Struct{
				"customer": fake.Company(),
				"estimate": fake.Number(1, 8),
			},
		}
		ticket := s.createTicketLocked(draft)
		ticket.Status = statuses[i%len(statuses)]
		ticket.Assignee = ptrPerson(people[(i+1)%len(people)])
		ticket.Watchers = []Person{people[(i+2)%len(people)], people[(i+3)%len(people)]}
		ticket.Source = TicketManualSource{ManualSource: "seed"}
		s.tickets[ticket.ID] = ticket
	}
}

func (s *InMemoryTicketService) createTicketLocked(draft TicketDraft) Ticket {
	s.nextTicket++
	projectID := draft.ProjectID
	if projectID == "" {
		projectID = DefaultProjectID
	}

	ticket := Ticket{
		ID:          fmt.Sprintf("TCK-%03d", s.nextTicket),
		ProjectID:   projectID,
		Title:       draft.Title,
		Description: draft.Description,
		Status:      TicketStatusOpen,
		Priority:    draft.Priority,
		Reporter:    draft.Reporter,
		Watchers:    []Person{draft.Reporter},
		Labels:      cloneLabelSet(draft.Labels),
		DueDate:     draft.DueDate,
		Visibility:  TicketVisibilityPublic,
		Metadata:    cloneStruct(draft.Metadata),
		Source:      TicketManualSource{ManualSource: "example"},
	}
	s.addEventLocked(&ticket, TicketEvent{
		Kind:    TicketEventKindCreated,
		Actor:   draft.Reporter,
		Note:    "Ticket created",
		Payload: tego.Value(map[string]any{"title": ticket.Title}),
	})
	s.tickets[ticket.ID] = ticket
	s.order = append(s.order, ticket.ID)
	return ticket
}

func (s *InMemoryTicketService) addEventLocked(ticket *Ticket, event TicketEvent) TicketEvent {
	event = s.prepareEventLocked(event)
	ticket.Events = append(ticket.Events, event)
	s.events = append(s.events, event)
	return event
}

func (s *InMemoryTicketService) addLooseEventLocked(event TicketEvent) TicketEvent {
	event = s.prepareEventLocked(event)
	s.events = append(s.events, event)
	return event
}

func (s *InMemoryTicketService) prepareEventLocked(event TicketEvent) TicketEvent {
	if event.ID == "" {
		s.nextEvent++
		event.ID = fmt.Sprintf("EVT-%03d", s.nextEvent)
	}
	if event.Kind == TicketEventKindUnspecified {
		event.Kind = TicketEventKindUpdated
	}
	if event.Actor.ID == "" {
		event.Actor = s.system
	}
	if event.Payload == nil {
		event.Payload = tego.Value(map[string]any{"source": "example"})
	}
	return event
}

func (s *InMemoryTicketService) eventsFor(projectID, ticketID string) []TicketEvent {
	if ticketID != "" {
		ticket, ok := s.tickets[ticketID]
		if !ok {
			return nil
		}
		return cloneEvents(ticket.Events)
	}

	if projectID == "" {
		projectID = DefaultProjectID
	}

	knownTickets := make(map[string]struct{})
	for _, ticket := range s.tickets {
		if ticket.ProjectID == projectID {
			knownTickets[ticket.ID] = struct{}{}
		}
	}

	events := make([]TicketEvent, 0, len(s.events))
	for _, ticket := range s.tickets {
		if _, ok := knownTickets[ticket.ID]; ok {
			events = append(events, cloneEvents(ticket.Events)...)
		}
	}
	if len(events) == 0 {
		events = cloneEvents(s.events)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].ID < events[j].ID
	})
	return events
}

func applyPatch(ticket *Ticket, patch TicketPatch) {
	if patch.Title.Valid {
		ticket.Title = patch.Title.Value
	}
	if patch.Description.Valid {
		ticket.Description = patch.Description.Value
	}
	if patch.Status.Valid {
		ticket.Status = patch.Status.Value
	}
	if patch.Priority.Valid {
		ticket.Priority = patch.Priority.Value
	}
	if patch.Assignee.Valid {
		ticket.Assignee = clonePersonPtr(patch.Assignee.Value)
	}
	if patch.DueDate.Valid {
		ticket.DueDate = patch.DueDate.Value
	}
	if patch.Labels.Valid {
		ticket.Labels = cloneLabelSet(patch.Labels.Value)
	}
	if patch.Metadata.Valid {
		ticket.Metadata = cloneStruct(patch.Metadata.Value)
	}
}

func matchesFilter(ticket Ticket, filter TicketFilter) bool {
	if len(filter.Statuses) > 0 {
		matched := false
		for _, status := range filter.Statuses {
			if ticket.Status == status {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if filter.Assignee != nil {
		if ticket.Assignee == nil || ticket.Assignee.ID != filter.Assignee.ID {
			return false
		}
	}
	for label := range filter.Labels {
		if _, ok := ticket.Labels[label]; !ok {
			return false
		}
	}
	return true
}

func ticketsByStatus(tickets []Ticket) map[TicketStatus][]Ticket {
	out := make(map[TicketStatus][]Ticket)
	for _, ticket := range tickets {
		out[ticket.Status] = append(out[ticket.Status], cloneTicket(ticket))
	}
	return out
}

func cloneTicket(ticket Ticket) Ticket {
	ticket.Assignee = clonePersonPtr(ticket.Assignee)
	ticket.Watchers = append([]Person(nil), ticket.Watchers...)
	ticket.Labels = cloneLabelSet(ticket.Labels)
	ticket.Metadata = cloneStruct(ticket.Metadata)
	ticket.Events = cloneEvents(ticket.Events)
	return ticket
}

func cloneEvents(events []TicketEvent) []TicketEvent {
	out := make([]TicketEvent, len(events))
	for i, event := range events {
		out[i] = event
		out[i].Attachments = append(tego.ListValue(nil), event.Attachments...)
	}
	return out
}

func cloneLabelSet(value types.Set[types.Label]) types.Set[types.Label] {
	if value == nil {
		return nil
	}
	out := make(types.Set[types.Label], len(value))
	for label := range value {
		out[label] = struct{}{}
	}
	return out
}

func cloneStruct(value tego.Struct) tego.Struct {
	if value == nil {
		return nil
	}
	out := make(tego.Struct, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func clonePersonPtr(person *Person) *Person {
	if person == nil {
		return nil
	}
	return ptrPerson(*person)
}

func ptrPerson(person Person) *Person {
	return &person
}

func labelSet(labels ...types.Label) types.Set[types.Label] {
	set := make(types.Set[types.Label], len(labels))
	for _, label := range labels {
		set[label] = struct{}{}
	}
	return set
}
