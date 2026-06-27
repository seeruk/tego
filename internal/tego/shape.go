package tego

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// ShapeIndex allows Tego's planner to quickly and easily identify if a protobuf message appears to
// be of a certain shape. These conventional shapes are used to produce cleaner Go code based on
// more expressive protobuf structures.
type ShapeIndex struct {
	Nullables map[protoreflect.FullName]*ProtoMessage
	Maps      map[protoreflect.FullName]*ProtoMessage
	Slices    map[protoreflect.FullName]*ProtoMessage
}

func BuildShapeIndex(di *DescriptorIndex) (*ShapeIndex, error) {
	builder := newShapeIndexBuilder()

	for _, f := range di.Files {
		if err := builder.indexFile(f); err != nil {
			return nil, fmt.Errorf("indexing file %s: %w", f.Path, err)
		}
	}

	return builder.index, nil
}

type shapeIndexBuilder struct {
	index  *ShapeIndex
	shapes map[*ProtoMessage]shapeKind
	active map[*ProtoMessage]bool
}

type shapeKind uint

const (
	shapeKindNone shapeKind = iota
	shapeKindNullable
	shapeKindMap
	shapeKindSlice
)

func newShapeIndexBuilder() *shapeIndexBuilder {
	return &shapeIndexBuilder{
		index: &ShapeIndex{
			Nullables: make(map[protoreflect.FullName]*ProtoMessage),
			Maps:      make(map[protoreflect.FullName]*ProtoMessage),
			Slices:    make(map[protoreflect.FullName]*ProtoMessage),
		},
		shapes: make(map[*ProtoMessage]shapeKind),
		active: make(map[*ProtoMessage]bool),
	}
}

func (b *shapeIndexBuilder) indexFile(file *ProtoFile) error {
	for _, message := range file.Messages {
		if err := b.indexMessage(message); err != nil {
			return fmt.Errorf("indexing message %s: %w", message.FullName, err)
		}
	}

	return nil
}

func (b *shapeIndexBuilder) indexMessage(message *ProtoMessage) error {
	switch b.classifyMessage(message) {
	case shapeKindNullable:
		b.index.Nullables[message.FullName] = message
	case shapeKindMap:
		b.index.Maps[message.FullName] = message
	case shapeKindSlice:
		b.index.Slices[message.FullName] = message
	}

	return nil
}

func (b *shapeIndexBuilder) classifyMessage(message *ProtoMessage) shapeKind {
	if message == nil {
		return shapeKindNone
	}
	if kind, ok := b.shapes[message]; ok {
		return kind
	}
	if b.active[message] {
		return shapeKindNone
	}

	b.active[message] = true
	defer delete(b.active, message)

	kind := shapeKindNone
	if !message.Options.HasInferShape() || message.Options.GetInferShape() {
		switch {
		case isNullableShape(message):
			kind = shapeKindNullable
		case b.isMapShape(message):
			kind = shapeKindMap
		case isSliceShape(message):
			kind = shapeKindSlice
		}
	}

	b.shapes[message] = kind
	return kind
}

func isNullableShape(message *ProtoMessage) bool {
	return isNullableOneofShape(message) || isNullableValueShape(message)
}

func isNullableOneofShape(message *ProtoMessage) bool {
	if len(message.Enums) > 0 || len(message.Messages) > 0 {
		return false
	}

	if len(message.Oneofs) != 1 || len(message.Fields) != 2 {
		return false
	}

	oneof := message.Oneofs[0]
	if len(oneof.Fields) != 2 {
		return false
	}

	for _, field := range oneof.Fields {
		if field.Enum != nil && field.Enum.FullName == "google.protobuf.NullValue" {
			return true
		}
	}

	return false
}

func isNullableValueShape(message *ProtoMessage) bool {
	if len(message.Enums) > 0 || len(message.Messages) > 0 || len(message.Oneofs) > 0 {
		return false
	}

	if len(message.Fields) != 2 {
		return false
	}

	var hasValid, hasValue bool
	for _, field := range message.Fields {
		switch {
		case field.Name == "value":
			hasValue = true
		case field.Name == "valid" && field.Kind == protoreflect.BoolKind:
			hasValid = true
		}
	}

	return hasValid && hasValue
}

func (b *shapeIndexBuilder) isMapShape(message *ProtoMessage) bool {
	if len(message.Enums) > 0 || len(message.Oneofs) > 0 {
		return false
	}

	if len(message.Fields) != 1 || len(message.Messages) != 1 {
		return false
	}

	mapMessage := message.Messages[0]
	if mapMessage.Name != "Map" {
		return false
	}

	field := message.Fields[0]
	if field.Cardinality != protoreflect.Repeated || field.Kind != protoreflect.MessageKind || field.Message != mapMessage {
		return false
	}

	key, value, ok := mapFields(mapMessage)
	if !ok || key == nil || value == nil {
		return false
	}

	return b.isComparableField(key)
}

func mapFields(message *ProtoMessage) (*ProtoField, *ProtoField, bool) {
	if len(message.Enums) > 0 || len(message.Messages) > 0 || len(message.Oneofs) > 0 {
		return nil, nil, false
	}

	if len(message.Fields) != 2 {
		return nil, nil, false
	}

	var key, value *ProtoField
	for _, field := range message.Fields {
		switch field.Name {
		case "key":
			key = field
		case "value":
			value = field
		default:
			return nil, nil, false
		}
	}

	return key, value, key != nil && value != nil
}

func (b *shapeIndexBuilder) isComparableField(field *ProtoField) bool {
	if field == nil {
		return false
	}

	if field.Cardinality == protoreflect.Repeated || isProtoMapField(field) {
		return false
	}

	if field.Options.HasGoType() {
		goType := field.Options.GetGoType()
		return goType.GetComparable() || goType.GetAsPointer()
	}

	switch field.Kind {
	case protoreflect.BoolKind,
		protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.FloatKind,
		protoreflect.Sfixed64Kind,
		protoreflect.Fixed64Kind,
		protoreflect.DoubleKind,
		protoreflect.StringKind,
		protoreflect.EnumKind:
		return true
	case protoreflect.BytesKind:
		return false
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return b.isComparableMessageField(field)
	default:
		return false
	}
}

func (b *shapeIndexBuilder) isComparableMessageField(field *ProtoField) bool {
	if field.Message == nil {
		return false
	}

	if field.Options.HasNullable() && field.Options.GetNullable() {
		return true
	}

	if field.Message.Options.HasGoType() {
		goType := field.Message.Options.GetGoType()
		return goType.GetComparable() || goType.GetAsPointer()
	}

	switch b.classifyMessage(field.Message) {
	case shapeKindNullable:
		return true
	case shapeKindMap, shapeKindSlice:
		return false
	default:
		return b.isComparableMessage(field.Message)
	}
}

func (b *shapeIndexBuilder) isComparableMessage(message *ProtoMessage) bool {
	if message == nil {
		return false
	}

	if b.active[message] {
		// AKA: A recursive type
		return false
	}

	b.active[message] = true
	defer delete(b.active, message)

	for _, field := range message.Fields {
		if !b.isComparableField(field) {
			return false
		}
	}

	return true
}

func isProtoMapField(field *ProtoField) bool {
	return field.MapKey != nil || field.MapValue != nil
}

func isSliceShape(message *ProtoMessage) bool {
	if len(message.Enums) > 0 || len(message.Messages) > 0 || len(message.Oneofs) > 0 {
		return false
	}

	if len(message.Fields) != 1 {
		return false
	}

	field := message.Fields[0]
	return field.Cardinality == protoreflect.Repeated
}
