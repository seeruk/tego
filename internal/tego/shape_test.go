package tego

import (
	"testing"

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
			"yirapb.v1.NullableNullablePeople",
		} {
			message := requireMessage(t, descriptorIndex, name)

			require.Contains(t, index.Nullables, name)
			assert.Same(t, message, index.Nullables[name])
		}
	})

	t.Run("indexes nullable value shapes", func(t *testing.T) {
		name := protoreflect.FullName("yirapb.v1.NullablePersonValue")
		message := requireMessage(t, descriptorIndex, name)

		require.Contains(t, index.Nullables, name)
		assert.Same(t, message, index.Nullables[name])
	})

	t.Run("does not index ordinary messages as nullable", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.Ticket",
			"yirapb.v1.Person",
			"yirapb.v1.People",
			"yirapb.v1.NullablePeople",
			"yirapb.v1.TicketsByPeople",
		} {
			requireMessage(t, descriptorIndex, name)
			assert.NotContains(t, index.Nullables, name)
		}
	})

	t.Run("indexes slice shapes", func(t *testing.T) {
		for _, name := range []protoreflect.FullName{
			"yirapb.v1.People",
			"yirapb.v1.NullablePeople",
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
			"yirapb.v1.NullablePerson",
			"yirapb.v1.NullablePersonValue",
			"yirapb.v1.NullableNullablePeople",
			"yirapb.v1.TicketsByPeople",
		} {
			requireMessage(t, descriptorIndex, name)
			assert.NotContains(t, index.Slices, name)
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

func field(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	return &ProtoField{
		Name: name,
		Kind: kind,
	}
}

func repeatedField(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.Cardinality = protoreflect.Repeated
	return field
}

func nullValueField(name protoreflect.Name) *ProtoField {
	return &ProtoField{
		Name: name,
		Kind: protoreflect.EnumKind,
		Enum: &ProtoEnum{FullName: "google.protobuf.NullValue"},
	}
}
