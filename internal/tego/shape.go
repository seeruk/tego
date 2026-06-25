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
	builder := &shapeIndexBuilder{
		index: &ShapeIndex{
			Nullables: make(map[protoreflect.FullName]*ProtoMessage),
			Maps:      make(map[protoreflect.FullName]*ProtoMessage),
			Slices:    make(map[protoreflect.FullName]*ProtoMessage),
		},
	}

	for _, f := range di.Files {
		if err := builder.indexFile(f); err != nil {
			return nil, fmt.Errorf("indexing file %s: %w", f.Path, err)
		}
	}

	return builder.index, nil
}

type shapeIndexBuilder struct {
	index *ShapeIndex
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
	if message.Options.HasInferShape() && !message.Options.GetInferShape() {
		return nil
	}

	switch {
	case isNullableShape(message):
		b.index.Nullables[message.FullName] = message
	case isMapShape(message):
		b.index.Maps[message.FullName] = message
	case isSliceShape(message):
		b.index.Slices[message.FullName] = message
	}

	return nil
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

func isMapShape(message *ProtoMessage) bool {
	return false
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
