package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestBuildShapeIndexYiraFixture(t *testing.T) {
	descriptorIndex := buildYiraDescriptorIndex(t)

	index, err := BuildShapeIndex(descriptorIndex)
	require.NoError(t, err)
	require.NotNil(t, index)

	t.Run("indexes nullable oneof shapes", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.NullablePerson",
		} {
			message := requireMessage(t, descriptorIndex, name)

			require.Contains(t, index.Nullables, name)
			assert.Same(t, message, index.Nullables[name])
		}
	})

	t.Run("does not index ordinary messages as nullable", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.Ticket",
			"yirapb.v1.Person",
			"yirapb.v1.PersonList",
			"yirapb.v1.TicketsByStatus",
		} {
			requireMessage(t, descriptorIndex, name)
			assert.NotContains(t, index.Nullables, name)
		}
	})

	t.Run("indexes slice shapes", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.PersonList",
		} {
			message := requireMessage(t, descriptorIndex, name)

			require.Contains(t, index.Slices, name)
			assert.Same(t, message, index.Slices[name])
		}
	})

	t.Run("does not index non-slice messages as slices", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.Ticket",
			"yirapb.v1.Person",
			"yirapb.v1.Labels",
			"yirapb.v1.NullablePerson",
			"yirapb.v1.TicketsByStatus",
		} {
			requireMessage(t, descriptorIndex, name)
			assert.NotContains(t, index.Slices, name)
		}
	})

	t.Run("indexes explicit flatten shapes", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.DueDate",
			"yirapb.v1.Labels",
		} {
			message := requireMessage(t, descriptorIndex, name)

			require.Contains(t, index.Flattens, name)
			assert.Same(t, message, index.Flattens[name])
		}
	})

	t.Run("indexes map shapes", func(t *testing.T) {
		name := protoreflect.FullName("yirapb.v1.TicketsByStatus")
		message := requireMessage(t, descriptorIndex, name)

		require.Contains(t, index.Maps, name)
		assert.Same(t, message, index.Maps[name])
	})

	t.Run("does not index non-map messages as maps", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.Ticket",
			"yirapb.v1.Person",
			"yirapb.v1.PersonList",
			"yirapb.v1.NullablePerson",
		} {
			requireMessage(t, descriptorIndex, name)
			assert.NotContains(t, index.Maps, name)
		}
	})
}

func TestBuildShapeIndexTegoCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		file      *ProtoFile
		indexed   bool
		indexName protoreflect.FullName
	}{
		{
			name:      "current generated file",
			file:      shapeIndexTestFile("generated.proto", true, "", "example.v1.GeneratedList"),
			indexed:   true,
			indexName: "example.v1.GeneratedList",
		},
		{
			name:      "external tego file",
			file:      shapeIndexTestFile("external.proto", false, "example.com/external;externalv1", "example.v1.ExternalList"),
			indexed:   true,
			indexName: "example.v1.ExternalList",
		},
		{
			name:      "plain imported file",
			file:      shapeIndexTestFile("foreign.proto", false, "", "example.v1.ForeignList"),
			indexed:   false,
			indexName: "example.v1.ForeignList",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			index, err := BuildShapeIndex(&DescriptorIndex{Files: []*ProtoFile{tt.file}})

			require.NoError(t, err)
			if tt.indexed {
				assert.Contains(t, index.Slices, tt.indexName)
			} else {
				assert.NotContains(t, index.Slices, tt.indexName)
			}
		})
	}
}

func TestNullableShapeHelpers(t *testing.T) {
	t.Run("nullable oneof requires exactly one oneof with two fields", func(t *testing.T) {
		person := field("person", protoreflect.MessageKind)
		null := nullValueField("null")
		valid := messageWithOneof(person, null)

		assert.True(t, isNullableOneofShape(valid))
		assert.False(t, isNullableOneofShape(messageWithFields(person, null)))
		assert.False(t, isNullableOneofShape(messageWithOneof(person, null, field("extra", protoreflect.StringKind))))
		assert.False(t, isNullableOneofShape(messageWithOneofAndExtraField([]*ProtoField{person, null}, field("extra", protoreflect.StringKind))))
		assert.False(t, isNullableOneofShape(messageWithNestedMessage(messageWithOneof(person, null))))
		assert.False(t, isNullableOneofShape(messageWithOneof(person, field("zero", protoreflect.EnumKind))))
	})

	t.Run("nullable value requires value and bool valid fields", func(t *testing.T) {
		value := field("value", protoreflect.MessageKind)
		valid := field("valid", protoreflect.BoolKind)

		assert.True(t, isNullableValueShape(messageWithFields(value, valid)))
		assert.False(t, isNullableValueShape(messageWithFields(valid, field("other", protoreflect.MessageKind))))
		assert.False(t, isNullableValueShape(messageWithFields(value, field("other", protoreflect.BoolKind))))
		assert.False(t, isNullableValueShape(messageWithFields(value, field("valid", protoreflect.StringKind))))
		assert.False(t, isNullableValueShape(messageWithFields(value, valid, field("extra", protoreflect.StringKind))))
		assert.False(t, isNullableValueShape(messageWithNestedMessage(messageWithFields(value, valid))))
		assert.False(t, isNullableValueShape(messageWithOneof(value, valid)))
	})
}

func TestMapShapeHelpers(t *testing.T) {
	t.Run("map requires repeated map entries with comparable key", func(t *testing.T) {
		builder := newShapeIndexBuilder()
		tests := []struct {
			name    string
			message *ProtoMessage
			want    bool
		}{
			{name: "string key", message: mapShapeWithKey(field("key", protoreflect.StringKind)), want: true},
			{name: "enum key", message: mapShapeWithKey(field("key", protoreflect.EnumKind)), want: true},
			{name: "comparable struct key", message: mapShapeWithKey(messageField("key", comparableStructMessage())), want: true},
			{name: "nullable oneof key", message: mapShapeWithKey(messageField("key", nullableOneofMessage())), want: true},
			{name: "nullable non-comparable struct key", message: mapShapeWithKey(nullableMessageField("key", nonComparableStructMessage())), want: true},
			{name: "comparable field go type key", message: mapShapeWithKey(fieldWithGoType("key", protoreflect.MessageKind, "Key", true)), want: true},
			{name: "pointer field go type key", message: mapShapeWithKey(fieldWithGoTypeAsPointer("key", protoreflect.MessageKind, "Key")), want: true},
			{name: "comparable message go type key", message: mapShapeWithKey(messageField("key", messageWithGoType("Key", true))), want: true},
			{name: "pointer message go type key", message: mapShapeWithKey(messageField("key", messageWithGoTypeAsPointer("Key"))), want: true},
			{name: "flattened string key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.StringKind)))), want: true},
			{name: "flattened enum key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.EnumKind)))), want: true},
			{name: "flattened comparable go type key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(fieldWithGoType("value", protoreflect.StringKind, "Key", true)))), want: true},
			{name: "flattened pointer go type key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(fieldWithGoTypeAsPointer("value", protoreflect.StringKind, "Key")))), want: true},
			{name: "flattened repeated comparable go type key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedFieldWithGoType("value", protoreflect.StringKind, "Key", true)))), want: true},
			{name: "bytes key", message: mapShapeWithKey(field("key", protoreflect.BytesKind))},
			{name: "repeated key", message: mapShapeWithKey(repeatedField("key", protoreflect.StringKind))},
			{name: "native proto map key", message: mapShapeWithKey(protoMapField("key"))},
			{name: "slice shape key", message: mapShapeWithKey(messageField("key", sliceShapeMessage()))},
			{name: "map shape key", message: mapShapeWithKey(messageField("key", mapShapeWithKey(field("key", protoreflect.StringKind))))},
			{name: "non-comparable struct key", message: mapShapeWithKey(messageField("key", nonComparableStructMessage()))},
			{name: "non-comparable field go type key", message: mapShapeWithKey(fieldWithGoType("key", protoreflect.MessageKind, "Key", false))},
			{name: "field go type without comparable option", message: mapShapeWithKey(fieldWithGoTypeRef("key", protoreflect.MessageKind, "Key"))},
			{name: "message go type without comparable option", message: mapShapeWithKey(messageField("key", messageWithGoTypeRef("Key")))},
			{name: "flattened bytes key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.BytesKind))))},
			{name: "flattened repeated key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedField("value", protoreflect.StringKind))))},
			{name: "flattened native proto map key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(protoMapField("value"))))},
			{name: "flattened repeated non-comparable go type key", message: mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedFieldWithGoType("value", protoreflect.StringKind, "Key", false))))},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, builder.isMapShape(tt.message))
			})
		}
	})

	t.Run("map rejects malformed wrappers", func(t *testing.T) {
		builder := newShapeIndexBuilder()
		mapMessage := mapEntryMessage(field("key", protoreflect.StringKind), field("value", protoreflect.StringKind))

		assert.False(t, builder.isMapShape(messageWithFields(repeatedMessageField("entries", mapMessage))))
		assert.False(t, builder.isMapShape(mapShapeWithEntryName("Entry")))
		assert.False(t, builder.isMapShape(messageWithMapEntry(mapMessage, field("entries", protoreflect.MessageKind))))
		assert.False(t, builder.isMapShape(messageWithMapEntry(mapMessage, repeatedMessageField("entries", &ProtoMessage{Name: "Other"}))))
		assert.False(t, builder.isMapShape(mapShapeWithEntryFields(field("key", protoreflect.StringKind))))
		assert.False(t, builder.isMapShape(mapShapeWithEntryFields(field("value", protoreflect.StringKind))))
		assert.False(t, builder.isMapShape(mapShapeWithEntryFields(
			field("key", protoreflect.StringKind),
			field("value", protoreflect.StringKind),
			field("extra", protoreflect.StringKind),
		)))
		assert.False(t, builder.isMapShape(messageWithMapEntry(
			messageWithNestedMessage(mapEntryMessage(field("key", protoreflect.StringKind), field("value", protoreflect.StringKind))),
			nil,
		)))
		assert.False(t, builder.isMapShape(messageWithMapEntry(
			messageWithNestedEnum(mapEntryMessage(field("key", protoreflect.StringKind), field("value", protoreflect.StringKind))),
			nil,
		)))
		assert.False(t, builder.isMapShape(messageWithMapEntry(
			mapEntryMessageWithOneof(field("key", protoreflect.StringKind), field("value", protoreflect.StringKind)),
			nil,
		)))
		assert.False(t, builder.isMapShape(messageWithOneof(repeatedMessageField("entries", mapMessage))))
	})
}

func TestSliceShapeHelpers(t *testing.T) {
	t.Run("slice requires exactly one repeated field", func(t *testing.T) {
		assert.True(t, isSliceShape(messageWithFields(repeatedField("strings", protoreflect.StringKind))))
		assert.True(t, isSliceShape(messageWithFields(repeatedField("messages", protoreflect.MessageKind))))
		assert.True(t, isSliceShape(messageWithFields(repeatedField("enums", protoreflect.EnumKind))))

		assert.False(t, isSliceShape(messageWithFields()))
		assert.False(t, isSliceShape(messageWithFields(field("value", protoreflect.MessageKind))))
		assert.False(t, isSliceShape(messageWithFields(
			repeatedField("values", protoreflect.MessageKind),
			field("other", protoreflect.StringKind),
		)))
		assert.False(t, isSliceShape(messageWithFields(
			repeatedField("values", protoreflect.MessageKind),
			repeatedField("others", protoreflect.StringKind),
		)))
		assert.False(t, isSliceShape(messageWithNestedMessage(messageWithFields(
			repeatedField("values", protoreflect.MessageKind),
		))))
		assert.False(t, isSliceShape(messageWithNestedEnum(messageWithFields(
			repeatedField("values", protoreflect.EnumKind),
		))))
		assert.False(t, isSliceShape(messageWithOneof(repeatedField("values", protoreflect.MessageKind))))
	})
}

func TestShapeIndexPrecedence(t *testing.T) {
	t.Run("message go type prevents shape indexing", func(t *testing.T) {
		message := messageWithFields(repeatedField("values", protoreflect.StringKind))
		message.FullName = "example.v1.Values"
		message.Options = &tegopb.MessageOptions{}
		message.Options.SetGoType(goTypeRef("github.com/example/project.Values"))

		builder := newShapeIndexBuilder()
		require.NoError(t, builder.indexMessage(message))

		assert.NotContains(t, builder.index.Slices, message.FullName)
	})

	t.Run("explicit flatten prevents inferred shape indexing", func(t *testing.T) {
		message := messageWithFields(repeatedField("values", protoreflect.StringKind))
		message.FullName = "example.v1.Values"
		message.Options = &tegopb.MessageOptions{}
		message.Options.SetFlatten(true)

		builder := newShapeIndexBuilder()
		require.NoError(t, builder.indexMessage(message))

		assert.Contains(t, builder.index.Flattens, message.FullName)
		assert.NotContains(t, builder.index.Slices, message.FullName)
	})
}
