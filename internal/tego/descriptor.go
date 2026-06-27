package tego

import (
	"fmt"

	"github.com/seeruk/tego/tegopb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DescriptorIndex is a navigable view of every protobuf descriptor passed to Tego.
type DescriptorIndex struct {
	Files []*ProtoFile

	FilesByPath      map[string]*ProtoFile
	MessagesByName   map[protoreflect.FullName]*ProtoMessage
	EnumsByName      map[protoreflect.FullName]*ProtoEnum
	EnumValuesByName map[protoreflect.FullName]*ProtoEnumValue
}

// ProtoFile describes one .proto file and the top-level declarations it owns.
type ProtoFile struct {
	Path     string
	Package  protoreflect.FullName
	Generate bool

	Messages []*ProtoMessage
	Enums    []*ProtoEnum

	Desc    *protogen.File
	Options *tegopb.FileOptions
}

// HasOptions reports whether this file has Tego-specific options.
func (f *ProtoFile) HasOptions() bool {
	return f.Options != nil
}

// ProtoMessage describes a protobuf message and its nested declarations.
type ProtoMessage struct {
	FullName protoreflect.FullName
	Name     protoreflect.Name
	GoName   string

	File   *ProtoFile
	Parent *ProtoMessage

	Fields   []*ProtoField
	Oneofs   []*ProtoOneof
	Messages []*ProtoMessage
	Enums    []*ProtoEnum

	Desc    *protogen.Message
	Options *tegopb.MessageOptions
}

// HasOptions reports whether this message has Tego-specific options.
func (m *ProtoMessage) HasOptions() bool {
	return m.Options != nil
}

// ProtoField describes a message field with links to its resolved type.
type ProtoField struct {
	FullName    protoreflect.FullName
	Name        protoreflect.Name
	GoName      string
	Number      protoreflect.FieldNumber
	Kind        protoreflect.Kind
	Cardinality protoreflect.Cardinality

	File     *ProtoFile
	Parent   *ProtoMessage
	Message  *ProtoMessage
	Enum     *ProtoEnum
	Oneof    *ProtoOneof
	MapKey   *ProtoField
	MapValue *ProtoField

	Desc    *protogen.Field
	Options *tegopb.FieldOptions
}

// HasOptions reports whether this field has Tego-specific options.
func (f *ProtoField) HasOptions() bool {
	return f.Options != nil
}

// HasPresence reports whether this field tracks explicit presence.
func (f *ProtoField) HasPresence() bool {
	if f.Desc == nil || f.Desc.Desc == nil {
		return false
	}
	return f.Desc.Desc.HasPresence()
}

// IsList reports whether this field is a repeated non-map field.
func (f *ProtoField) IsList() bool {
	if f.Desc == nil || f.Desc.Desc == nil {
		return f.Cardinality == protoreflect.Repeated && !f.IsMap()
	}
	return f.Desc.Desc.IsList()
}

// IsMap reports whether this field is a protobuf map field.
func (f *ProtoField) IsMap() bool {
	if f.Desc == nil || f.Desc.Desc == nil {
		return isProtoMapField(f)
	}
	return f.Desc.Desc.IsMap()
}

// IsRequired reports whether this field uses protobuf required cardinality.
func (f *ProtoField) IsRequired() bool {
	return f.Cardinality == protoreflect.Required
}

// ProtoOneof describes a protobuf oneof and the fields that belong to it.
type ProtoOneof struct {
	FullName protoreflect.FullName
	Name     protoreflect.Name
	GoName   string

	File   *ProtoFile
	Parent *ProtoMessage

	Fields []*ProtoField

	Desc *protogen.Oneof
}

// ProtoEnum describes a protobuf enum and its values.
type ProtoEnum struct {
	FullName protoreflect.FullName
	Name     protoreflect.Name
	GoName   string

	File   *ProtoFile
	Parent *ProtoMessage

	Values []*ProtoEnumValue

	Desc    *protogen.Enum
	Options *tegopb.EnumOptions
}

// HasOptions reports whether this enum has Tego-specific options.
func (e *ProtoEnum) HasOptions() bool {
	return e.Options != nil
}

// ProtoEnumValue describes one value declared by a protobuf enum.
type ProtoEnumValue struct {
	FullName protoreflect.FullName
	Name     protoreflect.Name
	GoName   string
	Number   protoreflect.EnumNumber

	File   *ProtoFile
	Parent *ProtoEnum

	Desc    *protogen.EnumValue
	Options *tegopb.EnumValueOptions
}

// HasOptions reports whether this enum value has Tego-specific options.
func (v *ProtoEnumValue) HasOptions() bool {
	return v.Options != nil
}

// BuildDescriptorIndex builds an indexed descriptor graph from a protogen plugin request.
func BuildDescriptorIndex(plugin *protogen.Plugin) (*DescriptorIndex, error) {
	builder := &descriptorIndexBuilder{
		index: &DescriptorIndex{
			FilesByPath:      make(map[string]*ProtoFile),
			MessagesByName:   make(map[protoreflect.FullName]*ProtoMessage),
			EnumsByName:      make(map[protoreflect.FullName]*ProtoEnum),
			EnumValuesByName: make(map[protoreflect.FullName]*ProtoEnumValue),
		},
		oneofsByName: make(map[protoreflect.FullName]*ProtoOneof),
	}

	for _, file := range plugin.Files {
		if err := builder.indexFile(file); err != nil {
			return nil, err
		}
	}

	if err := builder.finalizeFields(); err != nil {
		return nil, err
	}

	return builder.index, nil
}

type descriptorIndexBuilder struct {
	index        *DescriptorIndex
	oneofsByName map[protoreflect.FullName]*ProtoOneof
	fields       []*ProtoField
}

func (b *descriptorIndexBuilder) indexFile(desc *protogen.File) error {
	path := desc.Desc.Path()
	if _, exists := b.index.FilesByPath[path]; exists {
		return fmt.Errorf("duplicate proto file %q", path)
	}

	file := &ProtoFile{
		Path:     path,
		Package:  desc.Desc.Package(),
		Generate: desc.Generate,
		Desc:     desc,
		Options:  fileOptions(desc),
	}

	b.index.Files = append(b.index.Files, file)
	b.index.FilesByPath[path] = file

	for _, enum := range desc.Enums {
		registered, err := b.indexEnum(file, nil, enum)
		if err != nil {
			return err
		}
		file.Enums = append(file.Enums, registered)
	}

	for _, message := range desc.Messages {
		registered, err := b.indexMessage(file, nil, message)
		if err != nil {
			return err
		}
		file.Messages = append(file.Messages, registered)
	}

	return nil
}

func (b *descriptorIndexBuilder) indexEnum(file *ProtoFile, parent *ProtoMessage, desc *protogen.Enum) (*ProtoEnum, error) {
	fullName := desc.Desc.FullName()
	if _, exists := b.index.EnumsByName[fullName]; exists {
		return nil, fmt.Errorf("duplicate proto enum %q", fullName)
	}

	enum := &ProtoEnum{
		FullName: fullName,
		Name:     desc.Desc.Name(),
		GoName:   desc.GoIdent.GoName,
		File:     file,
		Parent:   parent,
		Desc:     desc,
		Options:  enumOptions(desc),
	}
	b.index.EnumsByName[fullName] = enum

	for _, value := range desc.Values {
		enumValue := &ProtoEnumValue{
			FullName: value.Desc.FullName(),
			Name:     value.Desc.Name(),
			GoName:   value.GoIdent.GoName,
			Number:   value.Desc.Number(),
			File:     file,
			Parent:   enum,
			Desc:     value,
			Options:  enumValueOptions(value),
		}
		enum.Values = append(enum.Values, enumValue)
		b.index.EnumValuesByName[enumValue.FullName] = enumValue
	}

	return enum, nil
}

func (b *descriptorIndexBuilder) indexMessage(file *ProtoFile, parent *ProtoMessage, desc *protogen.Message) (*ProtoMessage, error) {
	fullName := desc.Desc.FullName()
	if _, exists := b.index.MessagesByName[fullName]; exists {
		return nil, fmt.Errorf("duplicate proto message %q", fullName)
	}

	message := &ProtoMessage{
		FullName: fullName,
		Name:     desc.Desc.Name(),
		GoName:   desc.GoIdent.GoName,
		File:     file,
		Parent:   parent,
		Desc:     desc,
		Options:  messageOptions(desc),
	}
	b.index.MessagesByName[fullName] = message

	for _, oneof := range desc.Oneofs {
		registered, err := b.indexOneof(file, message, oneof)
		if err != nil {
			return nil, err
		}
		message.Oneofs = append(message.Oneofs, registered)
	}

	for _, field := range desc.Fields {
		registered := b.indexField(file, message, field)
		message.Fields = append(message.Fields, registered)
	}

	for _, enum := range desc.Enums {
		registered, err := b.indexEnum(file, message, enum)
		if err != nil {
			return nil, err
		}
		message.Enums = append(message.Enums, registered)
	}

	for _, nested := range desc.Messages {
		registered, err := b.indexMessage(file, message, nested)
		if err != nil {
			return nil, err
		}
		message.Messages = append(message.Messages, registered)
	}

	return message, nil
}

func (b *descriptorIndexBuilder) indexField(file *ProtoFile, parent *ProtoMessage, desc *protogen.Field) *ProtoField {
	field := &ProtoField{
		FullName:    desc.Desc.FullName(),
		Name:        desc.Desc.Name(),
		GoName:      desc.GoName,
		Number:      desc.Desc.Number(),
		Kind:        desc.Desc.Kind(),
		Cardinality: desc.Desc.Cardinality(),
		File:        file,
		Parent:      parent,
		Desc:        desc,
		Options:     fieldOptions(desc),
	}

	b.fields = append(b.fields, field)
	return field
}

func (b *descriptorIndexBuilder) indexOneof(file *ProtoFile, parent *ProtoMessage, desc *protogen.Oneof) (*ProtoOneof, error) {
	fullName := desc.Desc.FullName()
	if _, exists := b.oneofsByName[fullName]; exists {
		return nil, fmt.Errorf("duplicate proto oneof %q", fullName)
	}

	oneof := &ProtoOneof{
		FullName: fullName,
		Name:     desc.Desc.Name(),
		GoName:   desc.GoName,
		File:     file,
		Parent:   parent,
		Desc:     desc,
	}
	b.oneofsByName[fullName] = oneof
	return oneof, nil
}

func (b *descriptorIndexBuilder) finalizeFields() error {
	for _, field := range b.fields {
		if err := b.finalizeField(field); err != nil {
			return err
		}
	}
	return nil
}

func (b *descriptorIndexBuilder) finalizeField(field *ProtoField) error {
	desc := field.Desc.Desc

	if field.Desc.Oneof != nil {
		oneof, ok := b.oneofsByName[field.Desc.Oneof.Desc.FullName()]
		if !ok {
			return fmt.Errorf("field %q: no descriptor for oneof %q", field.FullName, field.Desc.Oneof.Desc.FullName())
		}
		field.Oneof = oneof
		oneof.Fields = append(oneof.Fields, field)
	}

	switch field.Kind {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		message, ok := b.index.MessagesByName[desc.Message().FullName()]
		if !ok {
			return fmt.Errorf("field %q: no descriptor for message %q", field.FullName, desc.Message().FullName())
		}
		field.Message = message
	case protoreflect.EnumKind:
		enum, ok := b.index.EnumsByName[desc.Enum().FullName()]
		if !ok {
			return fmt.Errorf("field %q: no descriptor for enum %q", field.FullName, desc.Enum().FullName())
		}
		field.Enum = enum
	}

	if field.IsMap() {
		if field.Message == nil {
			return fmt.Errorf("field %q: map field has no map entry message", field.FullName)
		}

		field.MapKey = protoFieldByName(field.Message, "key")
		if field.MapKey == nil {
			return fmt.Errorf("field %q: map entry message %q has no key field", field.FullName, field.Message.FullName)
		}

		field.MapValue = protoFieldByName(field.Message, "value")
		if field.MapValue == nil {
			return fmt.Errorf("field %q: map entry message %q has no value field", field.FullName, field.Message.FullName)
		}
	}

	return nil
}

func protoFieldByName(message *ProtoMessage, name protoreflect.Name) *ProtoField {
	for _, field := range message.Fields {
		if field.Name == name {
			return field
		}
	}
	return nil
}

func fileOptions(file *protogen.File) *tegopb.FileOptions {
	if opts := file.Desc.Options(); proto.HasExtension(opts, tegopb.E_File) {
		return proto.GetExtension(opts, tegopb.E_File).(*tegopb.FileOptions)
	}
	return nil
}

func enumOptions(enum *protogen.Enum) *tegopb.EnumOptions {
	if opts := enum.Desc.Options(); proto.HasExtension(opts, tegopb.E_Enum) {
		return proto.GetExtension(opts, tegopb.E_Enum).(*tegopb.EnumOptions)
	}
	return nil
}

func enumValueOptions(value *protogen.EnumValue) *tegopb.EnumValueOptions {
	if opts := value.Desc.Options(); proto.HasExtension(opts, tegopb.E_EnumValue) {
		return proto.GetExtension(opts, tegopb.E_EnumValue).(*tegopb.EnumValueOptions)
	}
	return nil
}

func messageOptions(message *protogen.Message) *tegopb.MessageOptions {
	if opts := message.Desc.Options(); proto.HasExtension(opts, tegopb.E_Message) {
		return proto.GetExtension(opts, tegopb.E_Message).(*tegopb.MessageOptions)
	}
	return nil
}

func fieldOptions(field *protogen.Field) *tegopb.FieldOptions {
	if opts := field.Desc.Options(); proto.HasExtension(opts, tegopb.E_Field) {
		return proto.GetExtension(opts, tegopb.E_Field).(*tegopb.FieldOptions)
	}
	return nil
}
