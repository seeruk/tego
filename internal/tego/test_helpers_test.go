package tego

import (
	"os"
	"strings"
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	plannerTestPkg           = "github.com/seeruk/tego/internal/tego/testdata/plannertest"
	wrapperspbTestImportPath = "google.golang.org/protobuf/types/known/wrapperspb"
)

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

func customTicketStatusGoType() *tegopb.GoType {
	goType := plannerGoType(
		plannerTestPkg+".CustomTicketStatus",
		plannerTestPkg+".CustomTicketStatusFromProto",
		plannerTestPkg+".CustomTicketStatusToProto",
		false,
	)
	goType.SetComparable(true)
	return goType
}

func planYiraWithCustomTicketStatus(t *testing.T) Plan {
	t.Helper()

	descriptors := buildYiraDescriptorIndex(t)
	status := requireEnum(t, descriptors, "yirapb.v1.TicketStatus")
	status.Options.SetGoType(customTicketStatusGoType())

	shapes, err := BuildShapeIndex(descriptors)
	require.NoError(t, err)
	plan, err := NewPlanner().Plan(descriptors, shapes)
	require.NoError(t, err)
	for _, file := range plan.Files {
		require.False(t, HasFatalDiagnostics(file.Diagnostics), diagnosticsText(file.Diagnostics))
	}
	return plan
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

func methodByName(t *testing.T, service *ProtoService, name protoreflect.Name) *ProtoMethod {
	t.Helper()

	for _, method := range service.Methods {
		if method.Name == name {
			return method
		}
	}

	t.Fatalf("method %q not found on service %q", name, service.FullName)
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

func requireService(t *testing.T, index *DescriptorIndex, name protoreflect.FullName) *ProtoService {
	t.Helper()

	service := index.ServicesByName[name]
	require.NotNil(t, service)
	return service
}

func protoFileWithOutput(protoPath, goPackage, outputPath string) *ProtoFile {
	options := &tegopb.FileOptions{}
	options.SetGoPackage(goPackage)
	if outputPath != "" {
		options.SetOutputPath(outputPath)
	}
	return &ProtoFile{
		Path:     protoPath,
		Generate: true,
		Options:  options,
	}
}

func testProtoFile(path string, generate bool, goPackage string) *ProtoFile {
	options := &tegopb.FileOptions{}
	if goPackage != "" {
		options.SetGoPackage(goPackage)
	}
	return &ProtoFile{
		Path:     path,
		Generate: generate,
		Options:  options,
	}
}

func omittedProtoFile(path string) *ProtoFile {
	file := testProtoFile(path, true, "")
	file.Options.SetOmit(true)
	return file
}

func attachMessagesToFile(file *ProtoFile, messages ...*ProtoMessage) {
	file.Messages = messages
	for _, message := range messages {
		attachMessageToFile(file, message)
	}
}

func attachMessageToFile(file *ProtoFile, message *ProtoMessage) {
	message.File = file
	for _, enum := range message.Enums {
		enum.File = file
	}
	for _, oneof := range message.Oneofs {
		oneof.File = file
		for _, field := range oneof.Fields {
			field.File = file
		}
	}
	for _, field := range message.Fields {
		field.File = file
	}
	for _, nested := range message.Messages {
		attachMessageToFile(file, nested)
	}
}

func attachServicesToFile(file *ProtoFile, services ...*ProtoService) {
	file.Services = services
	for _, service := range services {
		service.File = file
		for _, method := range service.Methods {
			method.File = file
			if method.Input != nil {
				method.Input.RPCInput = true
			}
			if method.Output != nil {
				method.Output.RPCOutput = true
			}
		}
	}
}

func plannerService(fullName protoreflect.FullName, name protoreflect.Name, methods ...*ProtoMethod) *ProtoService {
	service := &ProtoService{
		FullName: fullName,
		Name:     name,
		GoName:   string(name),
		Methods:  methods,
	}
	for _, method := range methods {
		method.Parent = service
	}
	return service
}

func plannerMethod(
	fullName protoreflect.FullName,
	name protoreflect.Name,
	input *ProtoMessage,
	output *ProtoMessage,
) *ProtoMethod {
	return &ProtoMethod{
		FullName: fullName,
		Name:     name,
		GoName:   string(name),
		Input:    input,
		Output:   output,
	}
}

func plannerMessage(fullName protoreflect.FullName, name protoreflect.Name) *ProtoMessage {
	return &ProtoMessage{
		FullName: fullName,
		Name:     name,
		Options:  &tegopb.MessageOptions{},
	}
}

func plannerEnum(fullName protoreflect.FullName, name protoreflect.Name, file *ProtoFile) *ProtoEnum {
	return &ProtoEnum{
		FullName: fullName,
		Name:     name,
		File:     file,
		Options:  &tegopb.EnumOptions{},
	}
}

func plannerOneof(parent *ProtoMessage, name protoreflect.Name, fields ...*ProtoField) *ProtoOneof {
	oneof := &ProtoOneof{
		FullName: protoreflect.FullName(string(parent.FullName) + "." + string(name)),
		Name:     name,
		GoName:   goName(string(name)),
		Parent:   parent,
		Fields:   fields,
	}
	parent.Oneofs = append(parent.Oneofs, oneof)
	parent.Fields = append(parent.Fields, fields...)
	for _, field := range fields {
		field.FullName = protoreflect.FullName(string(parent.FullName) + "." + string(field.Name))
		field.Parent = parent
		field.Oneof = oneof
	}
	return oneof
}

func field(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	return &ProtoField{
		Name: name,
		Kind: kind,
	}
}

func enumField(name protoreflect.Name, enum *ProtoEnum) *ProtoField {
	field := field(name, protoreflect.EnumKind)
	field.Enum = enum
	return field
}

func enumFieldWithGoType(name protoreflect.Name, comparable, asPointer, covered bool) *ProtoField {
	goPackage := ""
	if covered {
		goPackage = "example.com/tego;tego"
	}
	enum := plannerEnum("example.v1.Status", "Status", testProtoFile("status.proto", false, goPackage))
	goType := goTypeRef("example.com/types.Status")
	goType.SetComparable(comparable)
	goType.SetAsPointer(asPointer)
	enum.Options.SetGoType(goType)
	return enumField(name, enum)
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

func nullableOmittableField(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.Options = &tegopb.FieldOptions{}
	field.Options.SetNullable(true)
	field.Options.SetOmittable(true)
	return field
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

func fieldWithPlannerGoType(name protoreflect.Name, goType *tegopb.GoType) *ProtoField {
	field := field(name, protoreflect.StringKind)
	field.FullName = protoreflect.FullName("example.v1.Message." + name)
	options := &tegopb.FieldOptions{}
	options.SetGoType(goType)
	field.Options = options
	return field
}

func plannerGoType(ref, fromProto, toProto string, asPointer bool) *tegopb.GoType {
	goType := &tegopb.GoType{}
	goType.SetRef(ref)
	goType.SetFromProto(fromProto)
	goType.SetToProto(toProto)
	if asPointer {
		goType.SetAsPointer(true)
	}
	return goType
}

func plannerGoTypeWithArgs(ref string, args map[string]string, fromProto, toProto string, asPointer bool) *tegopb.GoType {
	goType := plannerGoType(ref, fromProto, toProto, asPointer)
	typeArgs := make(map[string]*tegopb.GoTypeArg, len(args))
	for name, typ := range args {
		arg := &tegopb.GoTypeArg{}
		arg.SetType(typ)
		typeArgs[name] = arg
	}
	goType.SetTypeArgs(typeArgs)
	return goType
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

func omittedPlannerField(field *ProtoField) *ProtoField {
	options := &tegopb.FieldOptions{}
	options.SetOmit(true)
	field.Options = options
	return field
}

func setServiceInlineByDefault(service *ProtoService, inlineByDefault bool) {
	options := &tegopb.ServiceOptions{}
	options.SetInlineByDefault(inlineByDefault)
	service.Options = options
}

func setMethodInline(method *ProtoMethod, inline, inlineRequest, inlineResponse *bool) {
	options := &tegopb.MethodOptions{}
	if inline != nil {
		options.SetInline(*inline)
	}
	if inlineRequest != nil {
		options.SetInlineRequest(*inlineRequest)
	}
	if inlineResponse != nil {
		options.SetInlineResponse(*inlineResponse)
	}
	method.Options = options
}

func setMessageFieldOptionsOmittable(options *tegopb.MessageOptions, omittable bool) {
	fields := &tegopb.MessageFieldsOptions{}
	fields.SetOmittable(omittable)
	options.SetFields(fields)
}

func setMessageFieldOptionsPreserveIntegerWidth(options *tegopb.MessageOptions, preserve bool) {
	fields := &tegopb.MessageFieldsOptions{}
	fields.SetPreserveIntegerWidth(preserve)
	options.SetFields(fields)
}

func setFieldOptionsPreserveIntegerWidth(field *ProtoField, preserve bool) {
	if field.Options == nil {
		field.Options = &tegopb.FieldOptions{}
	}
	field.Options.SetPreserveIntegerWidth(preserve)
}

func messageForCommentTest(protoName protoreflect.Name, goName, comment string) *ProtoMessage {
	return &ProtoMessage{
		FullName: protoreflect.FullName("example.v1." + protoName),
		Name:     protoName,
		GoName:   goName,
		Desc: &protogen.Message{
			Comments: protogen.CommentSet{Leading: protogen.Comments(comment)},
		},
		Options: &tegopb.MessageOptions{},
	}
}

func fieldForCommentTest(protoName protoreflect.Name, goName, comment string) *ProtoField {
	return &ProtoField{
		FullName: protoreflect.FullName("example.v1.Person." + protoName),
		Name:     protoName,
		GoName:   goName,
		Kind:     protoreflect.StringKind,
		Desc: &protogen.Field{
			Comments: protogen.CommentSet{Leading: protogen.Comments(comment)},
		},
		Options: &tegopb.FieldOptions{},
	}
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

func shapeIndexTestFile(path string, generate bool, goPackage string, fullName protoreflect.FullName) *ProtoFile {
	file := testProtoFile(path, generate, goPackage)
	message := plannerMessage(fullName, protoreflect.Name(fullName.Name()))
	message.File = file
	message.Fields = []*ProtoField{repeatedField("values", protoreflect.StringKind)}
	message.Fields[0].Parent = message
	file.Messages = []*ProtoMessage{message}
	return file
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

func wrapperMessage(fullName protoreflect.FullName, goName string) *ProtoMessage {
	message := plannerMessage(fullName, protoreflect.Name(goName))
	message.GoName = goName
	message.File = testProtoFile("google/protobuf/wrappers.proto", false, "")
	return message
}

func plannerMapShape() (*ProtoMessage, *ProtoMessage) {
	parent := plannerMessage("example.v1.StringsByName", "StringsByName")
	entry := plannerMessage("example.v1.StringsByName.Map", "Map")
	entry.Parent = parent
	entry.Fields = []*ProtoField{
		field("key", protoreflect.StringKind),
		field("value", protoreflect.Int64Kind),
	}
	for _, field := range entry.Fields {
		field.Parent = entry
	}
	parent.Messages = []*ProtoMessage{entry}
	parent.Fields = []*ProtoField{repeatedMessageField("entries", entry)}
	parent.Fields[0].Parent = parent
	return parent, entry
}

func enumByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) EnumPlan {
	t.Helper()

	for _, enum := range file.Enums {
		if enum.ProtoName == name {
			return enum
		}
	}

	t.Fatalf("enum %q not found", name)
	return EnumPlan{}
}

func enumConstantByProtoName(t *testing.T, enum EnumPlan, name protoreflect.FullName) EnumConstantPlan {
	t.Helper()

	for _, constant := range enum.Constants {
		if constant.ProtoName == name {
			return constant
		}
	}

	t.Fatalf("enum constant %q not found", name)
	return EnumConstantPlan{}
}

func structByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) StructPlan {
	t.Helper()

	for _, structure := range file.Structs {
		if structure.ProtoName == name {
			return structure
		}
	}

	t.Fatalf("struct %q not found", name)
	return StructPlan{}
}

func fieldPlanByProtoName(t *testing.T, structure StructPlan, name protoreflect.FullName) FieldPlan {
	t.Helper()

	for _, field := range structure.Fields {
		if field.ProtoName == name {
			return field
		}
	}

	t.Fatalf("field %q not found", name)
	return FieldPlan{}
}

func mappingByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) MappingPlan {
	t.Helper()

	for _, mapping := range file.Mappings {
		if mapping.ProtoName == name {
			return mapping
		}
	}

	t.Fatalf("mapping %q not found", name)
	return MappingPlan{}
}

func fieldMappingByProtoName(t *testing.T, mapping MappingPlan, name protoreflect.FullName) FieldMappingPlan {
	t.Helper()

	for _, field := range mapping.Fields {
		if field.ProtoName == name {
			return field
		}
	}

	t.Fatalf("field mapping %q not found", name)
	return FieldMappingPlan{}
}

func serviceByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) ServicePlan {
	t.Helper()

	for _, service := range file.Services {
		if service.ProtoName == name {
			return service
		}
	}

	t.Fatalf("service %q not found", name)
	return ServicePlan{}
}

func serviceMethodByProtoName(t *testing.T, service ServicePlan, name protoreflect.FullName) ServiceMethodPlan {
	t.Helper()

	for _, method := range service.Methods {
		if method.ProtoName == name {
			return method
		}
	}

	t.Fatalf("method %q not found", name)
	return ServiceMethodPlan{}
}

func inlineFieldNames(fields []ServiceInlineFieldPlan) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func requirePointerElem(t *testing.T, plan TypePlan, kind TypeKind) TypePlan {
	t.Helper()

	require.Equal(t, TypeKindPointer, plan.Kind)
	require.NotNil(t, plan.Elem)
	require.Equal(t, kind, plan.Elem.Kind)
	return *plan.Elem
}

func mustPlanFieldType(t *testing.T, field *ProtoField, shapeIndex *ShapeIndex) TypePlan {
	t.Helper()

	plan, diagnostics := NewPlanner().planSingularFieldType(field, shapeIndex)

	require.Empty(t, diagnostics)
	return plan
}

func requireFatalDiagnostic(t *testing.T, diagnostics []Diagnostic, contains string) {
	t.Helper()

	require.NotEmpty(t, diagnostics)
	assert.True(t, HasFatalDiagnostics(diagnostics))
	if contains != "" {
		assert.Contains(t, diagnosticsText(diagnostics), contains)
	}
}

func diagnosticsText(diagnostics []Diagnostic) string {
	var out strings.Builder
	for _, diagnostic := range diagnostics {
		out.WriteString(diagnostic.Message)
		out.WriteByte('\n')
	}
	return out.String()
}
