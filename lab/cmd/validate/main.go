package main

import (
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/davecgh/go-spew/spew"

	yirapbv1 "github.com/seeruk/tego/lab/yirapb/v1"
)

func main() {
	tkt := yirapbv1.Ticket_builder{
		Title:    new("Example Issue"),
		Status:   new(yirapbv1.TicketStatus_TICKET_STATUS_OPEN),
		Assignee: nil,
	}.Build()

	fmt.Println(tkt.HasAssignee())
	tkt.SetAssignee(nil)
	fmt.Println(tkt.HasAssignee())
	tkt.SetAssignee(yirapbv1.NullablePerson_builder{
		Person: yirapbv1.Person_builder{
			FirstName: new("Elliot"),
			LastName:  new("Wright"),
		}.Build(),
	}.Build())
	fmt.Println(tkt.HasAssignee())

	if tkt.HasAssignee() {
		assignee := tkt.GetAssignee()
		switch {
		case assignee.HasPerson():
			person := assignee.GetPerson()
			fmt.Println(person.GetFirstName())
		case assignee.HasNull():
			fmt.Println("null")
		}
	}

	err := protovalidate.Validate(tkt)
	spew.Dump(err)
}
