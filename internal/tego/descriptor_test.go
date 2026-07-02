package tego

import (
	"os"
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/pluginpb"
)

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

func buildYiraDescriptorIndex(t *testing.T) *DescriptorIndex {
	t.Helper()

	input, err := os.ReadFile("testdata/yira.codegenreq.bin")
	require.NoError(t, err)

	var req pluginpb.CodeGeneratorRequest
	require.NoError(t, proto.Unmarshal(input, &req))

	plugin, err := protogen.Options{}.New(&req)
	require.NoError(t, err)

	index, err := BuildDescriptorIndex(plugin)
	require.NoError(t, err)
	return index
}

func fieldByName(t *testing.T, message *ProtoMessage, name protoreflect.Name) *ProtoField {
	t.Helper()

	for _, field := range message.Fields {
		if field.Name == name {
			return field
		}
	}

	t.Fatalf("field %q not found on message %q", name, message.FullName)
	return nil
}

func requireFile(t *testing.T, index *DescriptorIndex, path string) *ProtoFile {
	t.Helper()

	file := index.FilesByPath[path]
	require.NotNil(t, file)
	return file
}

func requireMessage(t *testing.T, index *DescriptorIndex, name protoreflect.FullName) *ProtoMessage {
	t.Helper()

	message := index.MessagesByName[name]
	require.NotNil(t, message)
	return message
}

func requireEnum(t *testing.T, index *DescriptorIndex, name protoreflect.FullName) *ProtoEnum {
	t.Helper()

	enum := index.EnumsByName[name]
	require.NotNil(t, enum)
	return enum
}

func requireEnumValue(t *testing.T, index *DescriptorIndex, name protoreflect.FullName) *ProtoEnumValue {
	t.Helper()

	value := index.EnumValuesByName[name]
	require.NotNil(t, value)
	return value
}
