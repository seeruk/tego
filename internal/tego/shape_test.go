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

		assert.True(t, builder.isMapShape(mapShapeWithKey(field("key", protoreflect.StringKind))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(field("key", protoreflect.EnumKind))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", comparableStructMessage()))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", nullableOneofMessage()))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(nullableMessageField("key", nonComparableStructMessage()))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(fieldWithGoType("key", protoreflect.MessageKind, "Key", true))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(fieldWithGoTypeAsPointer("key", protoreflect.MessageKind, "Key"))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", messageWithGoType("Key", true)))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", messageWithGoTypeAsPointer("Key")))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.StringKind))))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.EnumKind))))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(fieldWithGoType("value", protoreflect.StringKind, "Key", true))))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(fieldWithGoTypeAsPointer("value", protoreflect.StringKind, "Key"))))))
		assert.True(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedFieldWithGoType("value", protoreflect.StringKind, "Key", true))))))

		assert.False(t, builder.isMapShape(mapShapeWithKey(field("key", protoreflect.BytesKind))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(repeatedField("key", protoreflect.StringKind))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(protoMapField("key"))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", sliceShapeMessage()))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", mapShapeWithKey(field("key", protoreflect.StringKind))))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", nonComparableStructMessage()))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(fieldWithGoType("key", protoreflect.MessageKind, "Key", false))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(fieldWithGoTypeRef("key", protoreflect.MessageKind, "Key"))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", messageWithGoTypeRef("Key")))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(field("value", protoreflect.BytesKind))))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedField("value", protoreflect.StringKind))))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(protoMapField("value"))))))
		assert.False(t, builder.isMapShape(mapShapeWithKey(messageField("key", flattenShapeMessage(repeatedFieldWithGoType("value", protoreflect.StringKind, "Key", false))))))
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

func messageWithFields(fields ...*ProtoField) *ProtoMessage {
	return &ProtoMessage{Fields: fields}
}

func messageWithOneof(fields ...*ProtoField) *ProtoMessage {
	oneof := &ProtoOneof{Fields: fields}
	message := messageWithFields(fields...)
	message.Oneofs = []*ProtoOneof{oneof}
	for _, field := range fields {
		field.Oneof = oneof
	}
	return message
}

func messageWithOneofAndExtraField(oneofFields []*ProtoField, extraFields ...*ProtoField) *ProtoMessage {
	message := messageWithOneof(oneofFields...)
	message.Fields = append(message.Fields, extraFields...)
	return message
}

func messageWithNestedMessage(message *ProtoMessage) *ProtoMessage {
	message.Messages = []*ProtoMessage{{}}
	return message
}

func messageWithNestedEnum(message *ProtoMessage) *ProtoMessage {
	message.Enums = []*ProtoEnum{{}}
	return message
}

func mapEntryMessageWithOneof(fields ...*ProtoField) *ProtoMessage {
	message := mapEntryMessage(fields...)
	message.Oneofs = []*ProtoOneof{{Fields: fields}}
	return message
}

func mapShapeWithKey(key *ProtoField) *ProtoMessage {
	return mapShapeWithEntryFields(key, field("value", protoreflect.StringKind))
}

func mapShapeWithEntryName(name protoreflect.Name) *ProtoMessage {
	mapMessage := mapEntryMessage(field("key", protoreflect.StringKind), field("value", protoreflect.StringKind))
	mapMessage.Name = name
	return messageWithMapEntry(mapMessage, nil)
}

func mapShapeWithEntryFields(fields ...*ProtoField) *ProtoMessage {
	return messageWithMapEntry(mapEntryMessage(fields...), nil)
}

func mapEntryMessage(fields ...*ProtoField) *ProtoMessage {
	return &ProtoMessage{
		Name:   "Map",
		Fields: fields,
	}
}

func messageWithMapEntry(mapMessage *ProtoMessage, entry *ProtoField) *ProtoMessage {
	if entry == nil {
		entry = repeatedMessageField("entries", mapMessage)
	}
	return &ProtoMessage{
		Fields:   []*ProtoField{entry},
		Messages: []*ProtoMessage{mapMessage},
	}
}

func comparableStructMessage() *ProtoMessage {
	return messageWithFields(
		field("name", protoreflect.StringKind),
		field("status", protoreflect.EnumKind),
	)
}

func nonComparableStructMessage() *ProtoMessage {
	return messageWithFields(field("data", protoreflect.BytesKind))
}

func nullableOneofMessage() *ProtoMessage {
	return messageWithOneof(field("person", protoreflect.MessageKind), nullValueField("null"))
}

func sliceShapeMessage() *ProtoMessage {
	return messageWithFields(repeatedField("values", protoreflect.StringKind))
}

func flattenShapeMessage(field *ProtoField) *ProtoMessage {
	options := &tegopb.MessageOptions{}
	options.SetFlatten(true)
	return &ProtoMessage{
		Fields:  []*ProtoField{field},
		Options: options,
	}
}

func messageWithGoType(ref string, comparable bool) *ProtoMessage {
	options := &tegopb.MessageOptions{}
	options.SetGoType(goType(ref, comparable))
	return &ProtoMessage{Options: options}
}

func messageWithGoTypeRef(ref string) *ProtoMessage {
	options := &tegopb.MessageOptions{}
	options.SetGoType(goTypeRef(ref))
	return &ProtoMessage{Options: options}
}

func messageWithGoTypeAsPointer(ref string) *ProtoMessage {
	options := &tegopb.MessageOptions{}
	options.SetGoType(goTypeAsPointer(ref))
	return &ProtoMessage{Options: options}
}

func field(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	return &ProtoField{
		Name: name,
		Kind: kind,
	}
}

func messageField(name protoreflect.Name, message *ProtoMessage) *ProtoField {
	field := field(name, protoreflect.MessageKind)
	field.Message = message
	return field
}

func repeatedMessageField(name protoreflect.Name, message *ProtoMessage) *ProtoField {
	field := repeatedField(name, protoreflect.MessageKind)
	field.Message = message
	return field
}

func nullableMessageField(name protoreflect.Name, message *ProtoMessage) *ProtoField {
	field := messageField(name, message)
	options := &tegopb.FieldOptions{}
	options.SetNullable(true)
	field.Options = options
	return field
}

func repeatedField(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.Cardinality = protoreflect.Repeated
	return field
}

func repeatedFieldWithGoType(name protoreflect.Name, kind protoreflect.Kind, ref string, comparable bool) *ProtoField {
	field := fieldWithGoType(name, kind, ref, comparable)
	field.Cardinality = protoreflect.Repeated
	return field
}

func protoMapField(name protoreflect.Name) *ProtoField {
	protoField := repeatedField(name, protoreflect.MessageKind)
	protoField.MapKey = field("key", protoreflect.StringKind)
	protoField.MapValue = field("value", protoreflect.StringKind)
	return protoField
}

func fieldWithGoType(name protoreflect.Name, kind protoreflect.Kind, ref string, comparable bool) *ProtoField {
	field := field(name, kind)
	options := &tegopb.FieldOptions{}
	options.SetGoType(goType(ref, comparable))
	field.Options = options
	return field
}

func fieldWithGoTypeRef(name protoreflect.Name, kind protoreflect.Kind, ref string) *ProtoField {
	field := field(name, kind)
	options := &tegopb.FieldOptions{}
	options.SetGoType(goTypeRef(ref))
	field.Options = options
	return field
}

func fieldWithGoTypeAsPointer(name protoreflect.Name, kind protoreflect.Kind, ref string) *ProtoField {
	field := field(name, kind)
	options := &tegopb.FieldOptions{}
	options.SetGoType(goTypeAsPointer(ref))
	field.Options = options
	return field
}

func goType(ref string, comparable bool) *tegopb.GoType {
	goType := &tegopb.GoType{}
	goType.SetRef(ref)
	goType.SetComparable(comparable)
	return goType
}

func goTypeRef(ref string) *tegopb.GoType {
	goType := &tegopb.GoType{}
	goType.SetRef(ref)
	return goType
}

func goTypeAsPointer(ref string) *tegopb.GoType {
	goType := goTypeRef(ref)
	goType.SetAsPointer(true)
	return goType
}

func nullValueField(name protoreflect.Name) *ProtoField {
	return &ProtoField{
		Name: name,
		Kind: protoreflect.EnumKind,
		Enum: &ProtoEnum{FullName: "google.protobuf.NullValue"},
	}
}
