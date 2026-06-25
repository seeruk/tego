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
		assert.Len(t, status.Values, 4)

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

		author := fieldByName(t, ticket, "author")
		assert.Equal(t, protoreflect.MessageKind, author.Kind)
		assert.Same(t, person, author.Message)

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
		ticketsByPeople := requireMessage(t, index, "yirapb.v1.TicketsByPeople")
		ticketsByPeopleMap := requireMessage(t, index, "yirapb.v1.TicketsByPeople.Map")

		assert.Same(t, ticketsByPeople, ticketsByPeopleMap.Parent)
		assert.Contains(t, ticketsByPeople.Messages, ticketsByPeopleMap)
	})

	t.Run("exposes protobuf map key and value fields", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		person := requireMessage(t, index, "yirapb.v1.Person")
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")

		metadata := fieldByName(t, ticket, "metadata")
		assert.True(t, metadata.IsMap())
		assert.Equal(t, protoreflect.StringKind, metadata.MapKey.Kind)
		assert.Equal(t, protoreflect.StringKind, metadata.MapValue.Kind)

		watchersByRole := fieldByName(t, ticket, "watchers_by_role")
		assert.True(t, watchersByRole.IsMap())
		assert.Equal(t, protoreflect.StringKind, watchersByRole.MapKey.Kind)
		assert.Equal(t, protoreflect.MessageKind, watchersByRole.MapValue.Kind)
		assert.Same(t, person, watchersByRole.MapValue.Message)

		workflowStatuses := fieldByName(t, ticket, "workflow_statuses")
		assert.True(t, workflowStatuses.IsMap())
		assert.Equal(t, protoreflect.StringKind, workflowStatuses.MapKey.Kind)
		assert.Equal(t, protoreflect.EnumKind, workflowStatuses.MapValue.Kind)
		assert.Same(t, status, workflowStatuses.MapValue.Enum)
	})

	t.Run("reports editions presence through field helpers", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		updateRequest := requireMessage(t, index, "yirapb.v1.UpdateTicketRequest")

		title := fieldByName(t, ticket, "title")
		assert.True(t, title.HasPresence())
		assert.False(t, title.IsList())
		assert.False(t, title.IsMap())
		assert.False(t, title.IsRequired())

		watcherIDs := fieldByName(t, ticket, "watcher_ids")
		assert.False(t, watcherIDs.HasPresence())
		assert.True(t, watcherIDs.IsList())
		assert.False(t, watcherIDs.IsMap())
		assert.False(t, watcherIDs.IsRequired())

		metadata := fieldByName(t, ticket, "metadata")
		assert.False(t, metadata.HasPresence())
		assert.False(t, metadata.IsList())
		assert.True(t, metadata.IsMap())
		assert.False(t, metadata.IsRequired())

		id := fieldByName(t, updateRequest, "id")
		assert.True(t, id.HasPresence())
		assert.False(t, id.IsList())
		assert.False(t, id.IsMap())
		assert.False(t, id.IsRequired())
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

		input := requireMessage(t, index, "yirapb.v1.TicketInput")
		require.True(t, input.HasOptions())
		require.NotNil(t, input.Options)
		assert.True(t, input.Options.HasFieldsOmittable())
		assert.True(t, input.Options.GetFieldsOmittable())
	})

	t.Run("resolves field options", func(t *testing.T) {
		ticket := requireMessage(t, index, "yirapb.v1.Ticket")
		input := requireMessage(t, index, "yirapb.v1.TicketInput")

		id := fieldByName(t, ticket, "id")
		require.True(t, id.HasOptions())
		require.Len(t, id.Options.GetTags(), 1)
		assert.Equal(t, "json", id.Options.GetTags()[0].GetKey())
		assert.Equal(t, "id,omitempty", id.Options.GetTags()[0].GetValue())

		title := fieldByName(t, ticket, "title")
		require.True(t, title.HasOptions())
		require.NotNil(t, title.Options.GetJsonTag())
		assert.Equal(t, "title", title.Options.GetJsonTag().GetValue())
		assert.True(t, title.Options.GetJsonTag().HasOmitempty())
		assert.True(t, title.Options.GetJsonTag().GetOmitempty())

		assignee := fieldByName(t, input, "assignee")
		require.True(t, assignee.HasOptions())
		assert.True(t, assignee.Options.HasNullable())
		assert.True(t, assignee.Options.GetNullable())

		version := fieldByName(t, input, "version")
		require.True(t, version.HasOptions())
		assert.True(t, version.Options.HasOmittable())
		assert.False(t, version.Options.GetOmittable())
	})

	t.Run("resolves enum options", func(t *testing.T) {
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")

		require.True(t, status.HasOptions())
		require.NotNil(t, status.Options)
		assert.Equal(t, "TicketStatus", status.Options.GetName())
		assert.Equal(t, "TicketStatus is the current lifecycle state of a ticket.", status.Options.GetComment())
		assert.True(t, status.Options.HasUnderlyingType())
		assert.Equal(t, tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_UINT, status.Options.GetUnderlyingType())
	})

	t.Run("resolves enum value options", func(t *testing.T) {
		unspecified := requireEnumValue(t, index, "yirapb.v1.TICKET_STATUS_UNSPECIFIED")
		assert.False(t, unspecified.HasOptions())
		assert.Nil(t, unspecified.Options)

		open := requireEnumValue(t, index, "yirapb.v1.TICKET_STATUS_OPEN")
		require.True(t, open.HasOptions())
		require.NotNil(t, open.Options)
		assert.Equal(t, "TicketStatusOpen", open.Options.GetName())
		assert.Equal(t, "TicketStatusOpen means work can begin.", open.Options.GetComment())
		assert.True(t, open.Options.HasUint())
		assert.Equal(t, uint64(1), open.Options.GetUint())
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
