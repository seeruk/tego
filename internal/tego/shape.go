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
	Flattens  map[protoreflect.FullName]*ProtoMessage
}

// BuildShapeIndex indexes all covered protobuf messages that Tego can flatten into Go shapes.
func BuildShapeIndex(di *DescriptorIndex) (*ShapeIndex, error) {
	builder := newShapeIndexBuilder()

	for _, f := range di.Files {
		if err := builder.indexFile(f); err != nil {
			return nil, fmt.Errorf("indexing file %s: %w", f.Path, err)
		}
	}

	return builder.index, nil
}

// withoutInferredShape removes automatic nullable/slice/map shape inference for
// one message while preserving explicit flatten entries.
func (si *ShapeIndex) withoutInferredShape(name protoreflect.FullName) *ShapeIndex {
	if si == nil || name == "" {
		return si
	}

	out := *si
	out.Nullables = shapeMapWithout(si.Nullables, name)
	out.Maps = shapeMapWithout(si.Maps, name)
	out.Slices = shapeMapWithout(si.Slices, name)
	return &out
}

func shapeMapWithout(
	shapes map[protoreflect.FullName]*ProtoMessage,
	name protoreflect.FullName,
) map[protoreflect.FullName]*ProtoMessage {
	if _, ok := shapes[name]; !ok {
		return shapes
	}

	out := make(map[protoreflect.FullName]*ProtoMessage, len(shapes)-1)
	for shapeName, message := range shapes {
		if shapeName != name {
			out[shapeName] = message
		}
	}
	return out
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
	shapeKindFlatten
)

func newShapeIndexBuilder() *shapeIndexBuilder {
	return &shapeIndexBuilder{
		index: &ShapeIndex{
			Nullables: make(map[protoreflect.FullName]*ProtoMessage),
			Maps:      make(map[protoreflect.FullName]*ProtoMessage),
			Slices:    make(map[protoreflect.FullName]*ProtoMessage),
			Flattens:  make(map[protoreflect.FullName]*ProtoMessage),
		},
		shapes: make(map[*ProtoMessage]shapeKind),
		active: make(map[*ProtoMessage]bool),
	}
}

func (b *shapeIndexBuilder) indexFile(file *ProtoFile) error {
	if !file.IsCoveredByTego() {
		return nil
	}

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
	case shapeKindFlatten:
		b.index.Flattens[message.FullName] = message
	}

	for _, nested := range message.Messages {
		if err := b.indexMessage(nested); err != nil {
			return fmt.Errorf("indexing message %s: %w", nested.FullName, err)
		}
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
	if message.Options.GetFlatten() && isFlattenShape(message) {
		kind = shapeKindFlatten
	} else if !message.Options.HasGoType() && (!message.Options.HasInferShape() || message.Options.GetInferShape()) {
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

func isFlattenShape(message *ProtoMessage) bool {
	_, ok := flattenShapeField(message)
	return ok
}

func flattenShapeField(message *ProtoMessage) (*ProtoField, bool) {
	if message == nil || len(message.Enums) > 0 || len(message.Messages) > 0 || len(message.Oneofs) > 0 || len(message.Fields) != 1 {
		return nil, false
	}
	field := message.Fields[0]
	return field, field.Oneof == nil
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
	if field.Kind == protoreflect.EnumKind {
		if goType, ok := coveredEnumGoType(field.Enum); ok {
			return goType.GetComparable() || goType.GetAsPointer()
		}
		return true
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
		protoreflect.StringKind:
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
		// Nullable message fields become pointers in generated Go, so they are comparable map keys.
		return true
	}

	if field.Message.Options.HasGoType() {
		goType := field.Message.Options.GetGoType()
		return goType.GetComparable() || goType.GetAsPointer()
	}

	switch b.classifyMessage(field.Message) {
	case shapeKindNullable:
		return true
	case shapeKindFlatten:
		flattened, ok := flattenShapeField(field.Message)
		return ok && b.isComparableFlattenField(flattened)
	case shapeKindMap, shapeKindSlice:
		return false
	default:
		return b.isComparableMessage(field.Message)
	}
}

func (b *shapeIndexBuilder) isComparableFlattenField(field *ProtoField) bool {
	if field == nil {
		return false
	}

	if field.Options.HasGoType() {
		goType := field.Options.GetGoType()
		return goType.GetComparable() || goType.GetAsPointer()
	}

	return b.isComparableField(field)
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
