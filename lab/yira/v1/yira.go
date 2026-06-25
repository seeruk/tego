package yirav1

import (
	"github.com/seeruk/tego/lab/omittable"
)

type TicketStatus uint

const (
	TicketStatusUnspecified TicketStatus = iota
	TicketStatusOpen
	TicketStatusInProgress
	TicketStatusClosed
)

type Ticket struct {
	ID          string
	Title       string
	Description string
	Status      TicketStatus
	Assignee    *Person
	Author      Person
	Version     string
}

type UpdateTicketRequest struct {
	ID    string
	Input TicketInput
}

type TicketInput struct {
	Title       omittable.Of[string]
	Description omittable.Of[string]
	Status      omittable.Of[TicketStatus]
	Assignee    omittable.Of[*Person]
	Version     string
}

type Person struct {
	FirstName string
	LastName  string
}
