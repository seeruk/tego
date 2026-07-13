package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestBuildDescriptorIndexFileOmitOption(t *testing.T) {
	tegoOptions := &tegopb.FileOptions{}
	tegoOptions.SetOmit(true)
	options := &descriptorpb.FileOptions{GoPackage: new("github.com/example/omittedpb;omittedpb")}
	proto.SetExtension(options, tegopb.E_File, tegoOptions)
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"omitted.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{{
			Name:    new("omitted.proto"),
			Package: new("example.v1"),
			Syntax:  new("editions"),
			Edition: descriptorpb.Edition_EDITION_2024.Enum(),
			Options: options,
		}},
	}
	plugin, err := protogen.Options{}.New(request)
	require.NoError(t, err)

	index, err := BuildDescriptorIndex(plugin)
	require.NoError(t, err)
	file := requireFile(t, index, "omitted.proto")

	require.NotNil(t, file.Options)
	assert.True(t, file.Options.HasOmit())
	assert.True(t, file.Options.GetOmit())
	assert.True(t, file.IsOmitted())
}

func TestBuildDescriptorIndexYiraFixture(t *testing.T) {
	index := buildYiraDescriptorIndex(t)

	t.Run("indexes generated and imported files", func(t *testing.T) {
		assert.NotEmpty(t, index.Files)
		assert.Greater(t, len(index.FilesByPath), 1)

		yiraFile := requireFile(t, index, "yirapb/v1/yira.proto")
		assert.True(t, yiraFile.Generate)

		structFile := requireFile(t, index, "google/protobuf/struct.proto")
		assert.False(t, structFile.Generate)

		optionsFile := requireFile(t, index, "tego/options.proto")
		assert.False(t, optionsFile.Generate)
	})

	t.Run("indexes declarations by full name", func(t *testing.T) {
		yiraFile := requireFile(t, index, "yirapb/v1/yira.proto")

		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		assert.Same(t, yiraFile, ticket.File)

		requireMessage(t, index, "yirapb.v1.NullablePerson")

		status := requireEnum(t, index, "yirapb.v1.TicketStatus")
		assert.Len(t, status.Values, 5)

		requireEnumValue(t, index, "yirapb.v1.TICKET_STATUS_OPEN")
	})

	t.Run("indexes services and methods", func(t *testing.T) {
		yiraFile := requireFile(t, index, "yirapb/v1/yira.proto")
		service := requireService(t, index, "yirapb.v1.TicketService")

		assert.Same(t, yiraFile, service.File)
		require.Len(t, service.Methods, 8)

		listTickets := methodByName(t, service, "ListTickets")
		assert.Same(t, requireMessage(t, index, "yirapb.v1.ListTicketsRequest"), listTickets.Input)
		assert.Same(t, requireMessage(t, index, "yirapb.v1.ListTicketsResponse"), listTickets.Output)
		assert.True(t, listTickets.Input.RPCInput)
		assert.False(t, listTickets.Input.RPCOutput)
		assert.True(t, listTickets.Input.IsRPCBoundary())
		assert.False(t, listTickets.Output.RPCInput)
		assert.True(t, listTickets.Output.RPCOutput)
		assert.True(t, listTickets.Output.IsRPCBoundary())
		assert.False(t, listTickets.ClientStreaming)
		assert.False(t, listTickets.ServerStreaming)

		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		assert.False(t, ticket.IsRPCBoundary())

		watchEvents := methodByName(t, service, "WatchTicketEvents")
		assert.Same(t, requireMessage(t, index, "yirapb.v1.WatchTicketEventsRequest"), watchEvents.Input)
		assert.Same(t, requireMessage(t, index, "yirapb.v1.WatchTicketEventsResponse"), watchEvents.Output)
		assert.False(t, watchEvents.ClientStreaming)
		assert.True(t, watchEvents.ServerStreaming)

		importEvents := methodByName(t, service, "ImportTicketEvents")
		assert.Same(t, requireMessage(t, index, "yirapb.v1.ImportTicketEventsRequest"), importEvents.Input)
		assert.Same(t, requireMessage(t, index, "yirapb.v1.ImportTicketEventsResponse"), importEvents.Output)
		assert.True(t, importEvents.ClientStreaming)
		assert.False(t, importEvents.ServerStreaming)

		syncEvents := methodByName(t, service, "SyncTicketEvents")
		assert.Same(t, requireMessage(t, index, "yirapb.v1.SyncTicketEventsRequest"), syncEvents.Input)
		assert.Same(t, requireMessage(t, index, "yirapb.v1.SyncTicketEventsResponse"), syncEvents.Output)
		assert.True(t, syncEvents.ClientStreaming)
		assert.True(t, syncEvents.ServerStreaming)
	})

	t.Run("resolves message and enum fields", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		nullablePerson := requireMessage(t, index, "yirapb.v1.NullablePerson")
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")
		person := requireMessage(t, index, "yirapb.v1.Person")

		assignee := fieldByName(t, ticket, "assignee")
		assert.Equal(t, protoreflect.MessageKind, assignee.Kind)
		assert.Same(t, nullablePerson, assignee.Message)

		reporter := fieldByName(t, ticket, "reporter")
		assert.Equal(t, protoreflect.MessageKind, reporter.Kind)
		assert.Same(t, person, reporter.Message)

		statusField := fieldByName(t, ticket, "status")
		assert.Equal(t, protoreflect.EnumKind, statusField.Kind)
		assert.Same(t, status, statusField.Enum)
	})

	t.Run("links oneof fields back to their oneof", func(t *testing.T) {
		nullablePerson := requireMessage(t, index, "yirapb.v1.NullablePerson")

		require.Len(t, nullablePerson.Oneofs, 1)
		valueOneof := nullablePerson.Oneofs[0]
		assert.Equal(t, protoreflect.Name("value"), valueOneof.Name)
		assert.Len(t, valueOneof.Fields, 2)
		assert.Same(t, valueOneof, fieldByName(t, nullablePerson, "person").Oneof)
		assert.Same(t, valueOneof, fieldByName(t, nullablePerson, "null").Oneof)
	})

	t.Run("preserves nested message parentage", func(t *testing.T) {
		ticketsByStatus := requireMessage(t, index, "yirapb.v1.TicketsByStatus")
		ticketsByStatusMap := requireMessage(t, index, "yirapb.v1.TicketsByStatus.Map")

		assert.Same(t, ticketsByStatus, ticketsByStatusMap.Parent)
		assert.Contains(t, ticketsByStatus.Messages, ticketsByStatusMap)
	})

	t.Run("resolves map shape key and value fields", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")
		ticketsByStatusMap := requireMessage(t, index, "yirapb.v1.TicketsByStatus.Map")

		key := fieldByName(t, ticketsByStatusMap, "key")
		assert.Equal(t, protoreflect.EnumKind, key.Kind)
		assert.Same(t, status, key.Enum)

		value := fieldByName(t, ticketsByStatusMap, "value")
		assert.Equal(t, protoreflect.MessageKind, value.Kind)
		assert.True(t, value.IsList())
		assert.Same(t, ticket, value.Message)
	})

	t.Run("reports editions presence through field helpers", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		updateRequest := requireMessage(t, index, "yirapb.v1.UpdateTicketRequest")

		title := fieldByName(t, ticket, "title")
		assert.True(t, title.HasPresence())
		assert.False(t, title.IsList())
		assert.False(t, title.IsMap())
		assert.False(t, title.IsRequired())

		labels := fieldByName(t, ticket, "labels")
		assert.True(t, labels.HasPresence())
		assert.False(t, labels.IsList())
		assert.False(t, labels.IsMap())
		assert.False(t, labels.IsRequired())

		metadata := fieldByName(t, ticket, "metadata")
		assert.True(t, metadata.HasPresence())
		assert.False(t, metadata.IsList())
		assert.False(t, metadata.IsMap())
		assert.False(t, metadata.IsRequired())

		ticketID := fieldByName(t, updateRequest, "ticket_id")
		assert.True(t, ticketID.HasPresence())
		assert.False(t, ticketID.IsList())
		assert.False(t, ticketID.IsMap())
		assert.False(t, ticketID.IsRequired())
	})

	t.Run("resolves file options", func(t *testing.T) {
		yiraFile := requireFile(t, index, "yirapb/v1/yira.proto")

		require.True(t, yiraFile.HasOptions())
		require.NotNil(t, yiraFile.Options)
		assert.True(t, yiraFile.Options.HasGoPackage())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1;yirav1", yiraFile.Options.GetGoPackage())

		structFile := requireFile(t, index, "google/protobuf/struct.proto")
		assert.False(t, structFile.HasOptions())
		assert.Nil(t, structFile.Options)
	})

	t.Run("resolves message options", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		assert.False(t, ticket.HasOptions())
		assert.Nil(t, ticket.Options)

		patch := requireMessage(t, index, "yirapb.v1.TicketPatch")
		require.True(t, patch.HasOptions())
		require.NotNil(t, patch.Options)
		require.True(t, patch.Options.HasFields())
		assert.True(t, patch.Options.GetFields().GetOmittable())

		for _, name := range []protoreflect.FullName{
			"yirapb.v1.DueDate",
			"yirapb.v1.Labels",
		} {
			message := requireMessage(t, index, name)
			require.True(t, message.HasOptions())
			require.NotNil(t, message.Options)
			assert.True(t, message.Options.HasFlatten())
			assert.True(t, message.Options.GetFlatten())
		}
	})

	t.Run("resolves field options", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		patch := requireMessage(t, index, "yirapb.v1.TicketPatch")

		id := fieldByName(t, ticket, "id")
		require.True(t, id.HasOptions())
		require.NotNil(t, id.Options.GetJsonTag())
		assert.Equal(t, "id", id.Options.GetJsonTag().GetValue())
		assert.True(t, id.Options.GetJsonTag().HasOmitempty())
		assert.True(t, id.Options.GetJsonTag().GetOmitempty())

		dueDate := requireMessage(t, index, "yirapb.v1.DueDate")
		value := fieldByName(t, dueDate, "value")
		require.True(t, value.HasOptions())
		require.NotNil(t, value.Options.GetGoType())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.Date", value.Options.GetGoType().GetRef())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.DateFromProto", value.Options.GetGoType().GetFromProto())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.DateToProto", value.Options.GetGoType().GetToProto())
		assert.True(t, value.Options.GetGoType().HasComparable())
		assert.True(t, value.Options.GetGoType().GetComparable())

		goType := &tegopb.GoType{}
		assert.False(t, goType.HasAsPointer())
		assert.False(t, goType.GetAsPointer())

		labels := requireMessage(t, index, "yirapb.v1.Labels")
		values := fieldByName(t, labels, "values")
		require.True(t, values.HasOptions())
		require.NotNil(t, values.Options.GetGoType())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.Set[T]", values.Options.GetGoType().GetRef())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.LabelSetFromProto", values.Options.GetGoType().GetFromProto())
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.LabelSetToProto", values.Options.GetGoType().GetToProto())
		require.Contains(t, values.Options.GetGoType().GetTypeArgs(), "T")
		assert.Equal(
			t,
			"github.com/seeruk/tego/internal/tego/testdata/yira/v1/types.Label",
			values.Options.GetGoType().GetTypeArgs()["T"].GetType(),
		)

		version := fieldByName(t, patch, "version")
		require.True(t, version.HasOptions())
		assert.True(t, version.Options.HasOmittable())
		assert.False(t, version.Options.GetOmittable())
	})

	t.Run("resolves service and method options", func(t *testing.T) {
		service := requireService(t, index, "yirapb.v1.TicketService")
		require.True(t, service.HasOptions())
		require.NotNil(t, service.Options)
		assert.True(t, service.Options.HasName())
		assert.Equal(t, "TicketService", service.Options.GetName())
		assert.True(t, service.Options.HasComment())
		assert.Equal(t, "TicketService is the facade API for Yira tickets.", service.Options.GetComment())
		assert.True(t, service.Options.HasInlineByDefault())
		assert.True(t, service.Options.GetInlineByDefault())

		listTickets := methodByName(t, service, "ListTickets")
		assert.False(t, listTickets.HasOptions())
		assert.Nil(t, listTickets.Options)

		getTicket := methodByName(t, service, "GetTicket")
		require.True(t, getTicket.HasOptions())
		assert.True(t, getTicket.Options.HasName())
		assert.Equal(t, "GetTicket", getTicket.Options.GetName())
		assert.True(t, getTicket.Options.HasComment())
		assert.Equal(t, "GetTicket fetches a ticket by ID.", getTicket.Options.GetComment())
		assert.True(t, getTicket.Options.HasInline())
		assert.True(t, getTicket.Options.GetInline())

		updateTicket := methodByName(t, service, "UpdateTicket")
		assert.False(t, updateTicket.HasOptions())
		assert.Nil(t, updateTicket.Options)
	})

	t.Run("resolves enum options", func(t *testing.T) {
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")

		require.True(t, status.HasOptions())
		require.NotNil(t, status.Options)
		assert.Equal(t, "TicketStatus", status.Options.GetName())
		assert.Empty(t, status.Options.GetComment())
		assert.True(t, status.Options.HasUnderlyingType())
		assert.Equal(t, tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_UINT, status.Options.GetUnderlyingType())
	})

	t.Run("resolves enum value options", func(t *testing.T) {
		unspecified := requireEnumValue(t, index, "yirapb.v1.TICKET_STATUS_UNSPECIFIED")
		require.True(t, unspecified.HasOptions())
		require.NotNil(t, unspecified.Options)
		assert.Equal(t, "TicketStatusUnknown", unspecified.Options.GetName())
		assert.Equal(t, "TicketStatus is the current lifecycle state of a ticket.", unspecified.Options.GetComment())
		assert.True(t, unspecified.Options.HasUint())
		assert.Equal(t, uint64(0), unspecified.Options.GetUint())

		open := requireEnumValue(t, index, "yirapb.v1.TICKET_STATUS_OPEN")
		assert.False(t, open.HasOptions())
		assert.Nil(t, open.Options)
	})
}
