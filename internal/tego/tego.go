package tego

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Plan is the complete generation plan produced by the planner.
type Plan struct {
	Files []FilePlan
}

// FilePlan describes everything Tego will emit for one generated protobuf file.
type FilePlan struct {
	ProtoPath             string
	Output                FileOutputPlan
	Package               PackageRef
	Enums                 []EnumPlan
	Oneofs                []OneofPlan
	Structs               []StructPlan
	Mappings              []MappingPlan
	Services              []ServicePlan
	RequestInlineHelpers  []ServiceInlineHelperPlan
	ResponseInlineHelpers []ServiceInlineHelperPlan
	Diagnostics           []Diagnostic
}

// FileOutputPlan describes the generated Go file path before and after module stripping.
type FileOutputPlan struct {
	// Directory is the generated file directory relative to the plugin output root.
	Directory string

	// Filename is the generated Go filename.
	Filename string

	// Path is the joined generated file path relative to the plugin output root.
	Path string

	// GeneratorPath is the path passed to protogen.Plugin.NewGeneratedFile before protogen applies module stripping.
	GeneratorPath string
}

// PackageRef identifies a Go package by import path and package name.
type PackageRef struct {
	ImportPath string
	Name       string
}

// EnumPlan describes a generated Go enum and its constants.
type EnumPlan struct {
	ProtoName  protoreflect.FullName
	Name       string
	Comment    string
	Underlying EnumUnderlyingType
	Constants  []EnumConstantPlan
}

// EnumUnderlyingType enumerates the possible Go numeric/string representations used for a generated
// enum.
type EnumUnderlyingType uint

const (
	// EnumUnderlyingTypeUint renders the enum as a Go uint.
	EnumUnderlyingTypeUint EnumUnderlyingType = iota
	// EnumUnderlyingTypeInt renders the enum as a Go int.
	EnumUnderlyingTypeInt
	// EnumUnderlyingTypeString renders the enum as a Go string.
	EnumUnderlyingTypeString
)

// EnumConstantPlan describes one generated Go enum constant.
type EnumConstantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Value     EnumConstantValue
}

// EnumConstantValue stores the planned literal for an enum constant.
type EnumConstantValue struct {
	Uint   uint
	Int    int
	String string
}

// OneofPlan describes the generated Go interface and variants for a protobuf oneof.
type OneofPlan struct {
	ProtoName    protoreflect.FullName
	Name         string
	Comment      string
	MarkerMethod string
	Variants     []OneofVariantPlan
}

// OneofVariantPlan describes one concrete Go variant for a protobuf oneof field.
type OneofVariantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	FieldName string
	Comment   string
	Type      TypePlan
}

// StructPlan describes a generated Go struct for a protobuf message.
type StructPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Fields    []FieldPlan
}

// FieldPlan describes one generated Go struct field.
type FieldPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Type      TypePlan
	Tags      []StructTagPlan
}

// TypePlan is Tego's language for a Go type before it is rendered.
type TypePlan struct {
	Kind   TypeKind
	Scalar ScalarKind
	Ref    GoTypeRef
	Custom CustomGoTypePlan
	Elem   *TypePlan
	Key    *TypePlan
	Value  *TypePlan
}

// TypeKind classifies the shape of a TypePlan.
type TypeKind uint

const (
	// TypeKindScalar is a built-in Go scalar.
	TypeKindScalar TypeKind = iota
	// TypeKindEnum is a Tego-generated enum.
	TypeKindEnum
	// TypeKindStruct is a Tego-generated struct.
	TypeKindStruct
	// TypeKindExternal is a Go type generated or owned outside the current Tego package.
	TypeKindExternal
	// TypeKindCustom is a user-specified Go type with custom mapping functions.
	TypeKindCustom
	// TypeKindOneof is a Tego-generated oneof interface.
	TypeKindOneof
	// TypeKindPointer is a pointer to another planned type.
	TypeKindPointer
	// TypeKindSlice is a slice of another planned type.
	TypeKindSlice
	// TypeKindMap is a Go map with planned key and value types.
	TypeKindMap
	// TypeKindOmittable is an omittable wrapper around another planned type.
	TypeKindOmittable
	// TypeKindEmptyStruct is the Go struct{} representation used for google.protobuf.Empty.
	TypeKindEmptyStruct
)

// ScalarKind classifies scalar Go types used in TypePlan.
type ScalarKind uint

const (
	ScalarKindBool        ScalarKind = iota
	ScalarKindInt32                  // from int32, sint32, sfixed32
	ScalarKindInt64                  // from int64, sint64, sfixed64
	ScalarKindUint32                 // from uint32, fixed32
	ScalarKindUint64                 // from uint64, fixed64
	ScalarKindFixedInt64             // from int64, sint64, sfixed64 with preserved integer width
	ScalarKindFixedUint64            // from uint64, fixed64 with preserved integer width
	ScalarKindFloat32                // from float
	ScalarKindFloat64                // from double
	ScalarKindString                 // from string
	ScalarKindBytes                  // from bytes
	ScalarKindAny                    // from well-known dynamic values
)

// GoTypeRef identifies a Go type expression.
type GoTypeRef struct {
	ImportPath string
	Name       string
	Args       []GoTypeRef
	Pointer    *GoTypeRef
	Slice      *GoTypeRef
	Array      *GoTypeRef
	ArrayLen   int64
	MapKey     *GoTypeRef
	MapValue   *GoTypeRef
}

// CustomGoTypePlan records a user-provided Go type and its conversion functions.
type CustomGoTypePlan struct {
	Ref               GoTypeRef
	FromProto         GoSymbolRef
	FromProtoCanError bool
	ToProto           GoSymbolRef
	ToProtoCanError   bool
}

// GoSymbolRef identifies a Go function or method that may need an import qualifier.
type GoSymbolRef struct {
	ImportPath string
	Name       string
	Receiver   string
}

// StructTagPlan describes one generated struct tag key/value pair.
type StructTagPlan struct {
	Key   string
	Value string
}

// MappingPlan describes the generated conversion functions for one message.
type MappingPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	ProtoRef  GoTypeRef
	Type      TypePlan
	FromProto MappingFunctionPlan
	ToProto   MappingFunctionPlan
	Fields    []FieldMappingPlan
}

// MappingFunctionPlan describes a generated top-level conversion function.
type MappingFunctionPlan struct {
	Name         string
	ReceiverName string
	Source       TypePlan
	Target       TypePlan
	CanError     bool
}

// FieldMappingPlan describes how one field maps between protobuf and Tego forms.
type FieldMappingPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Proto     MappingFieldAccessPlan
	FromProto MappingValuePlan
	ToProto   MappingValuePlan
}

// MappingValuePlan describes one value-level conversion expression or block.
type MappingValuePlan struct {
	Kind      MappingValueKind
	Source    TypePlan
	Target    TypePlan
	CanError  bool
	Access    MappingAccessPlan
	Oneof     *MappingOneofPlan
	Struct    *MappingRefPlan
	Custom    *CustomGoTypePlan
	Enum      *MappingEnumPlan
	Cast      *MappingCastPlan
	Dynamic   *MappingDynamicPlan
	WellKnown *MappingWellKnownPlan
	Elem      *MappingValuePlan
	Key       *MappingValuePlan
	Value     *MappingValuePlan
}

// MappingAccessPlan records generated accessor names needed by mapping renderers.
type MappingAccessPlan struct {
	Field         MappingFieldAccessPlan
	Key           MappingFieldAccessPlan
	Value         MappingFieldAccessPlan
	Valid         MappingFieldAccessPlan
	Oneof         MappingOneofAccessPlan
	NullableForm  MappingNullableForm
	ProtoType     TypePlan
	ProtoElemType TypePlan
}

// MappingFieldAccessPlan records protobuf-style getter, setter, presence, and clear names.
type MappingFieldAccessPlan struct {
	Name   string
	Getter string
	Setter string
	Has    string
	Clear  string
}

// MappingOneofAccessPlan records the generated API surface for a nullable oneof shape.
type MappingOneofAccessPlan struct {
	Name     string
	Which    string
	Value    MappingFieldAccessPlan
	Null     MappingFieldAccessPlan
	ValueRef GoTypeRef
	NullRef  GoTypeRef
}

// MappingOneofPlan describes conversion for a regular protobuf oneof.
type MappingOneofPlan struct {
	Which    string
	Variants []MappingOneofVariantPlan
}

// MappingOneofVariantPlan describes conversion for one protobuf oneof case.
type MappingOneofVariantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	FieldName string
	Proto     MappingFieldAccessPlan
	Case      GoTypeRef
	Value     MappingValuePlan
}

// MappingNullableForm describes how nullability is represented at the protobuf boundary.
type MappingNullableForm uint

const (
	// MappingNullableFormNone means the value is not nullable at this mapping point.
	MappingNullableFormNone MappingNullableForm = iota
	// MappingNullableFormPointer means nil is represented by a Go pointer.
	MappingNullableFormPointer
	// MappingNullableFormOneof means nil is represented by a protobuf nullable oneof shape.
	MappingNullableFormOneof
	// MappingNullableFormValue means nil is represented by a protobuf nullable value/valid shape.
	MappingNullableFormValue
)

// MappingValueKind selects the renderer used for a MappingValuePlan.
type MappingValueKind uint

const (
	// MappingValueKindUnsupported records a mapping the renderer cannot emit.
	MappingValueKindUnsupported MappingValueKind = iota
	// MappingValueKindDirect maps a value without conversion.
	MappingValueKindDirect
	// MappingValueKindScalarCast maps by rendering a Go scalar cast.
	MappingValueKindScalarCast
	// MappingValueKindEnum maps between enum representations.
	MappingValueKindEnum
	// MappingValueKindStruct maps by calling another struct mapping function.
	MappingValueKindStruct
	// MappingValueKindCustom maps by calling a user-provided conversion function.
	MappingValueKindCustom
	// MappingValueKindNullable maps nullable pointer or shape values.
	MappingValueKindNullable
	// MappingValueKindSlice maps each element of a slice.
	MappingValueKindSlice
	// MappingValueKindMap maps each key and value in a map.
	MappingValueKindMap
	// MappingValueKindOmittable maps an omittable wrapper while preserving presence.
	MappingValueKindOmittable
	// MappingValueKindOneof maps a protobuf oneof to a Tego oneof interface.
	MappingValueKindOneof
	// MappingValueKindEmptyStruct maps google.protobuf.Empty to struct{}.
	MappingValueKindEmptyStruct
	// MappingValueKindDynamic maps Struct, Value, or ListValue well-known dynamic data.
	MappingValueKindDynamic
	// MappingValueKindWellKnown maps timestamp or duration well-known types.
	MappingValueKindWellKnown
	// MappingValueKindFlatten maps an explicit one-field flatten shape.
	MappingValueKindFlatten
)

// MappingDynamicKind identifies which dynamic protobuf well-known type is being mapped.
type MappingDynamicKind uint

const (
	// MappingDynamicKindStruct maps google.protobuf.Struct.
	MappingDynamicKindStruct MappingDynamicKind = iota
	// MappingDynamicKindValue maps google.protobuf.Value.
	MappingDynamicKindValue
	// MappingDynamicKindListValue maps google.protobuf.ListValue.
	MappingDynamicKindListValue
)

// MappingDynamicPlan carries dynamic well-known type mapping details.
type MappingDynamicPlan struct {
	Kind MappingDynamicKind
}

// MappingWellKnownKind identifies which non-dynamic protobuf well-known type is being mapped.
type MappingWellKnownKind uint

const (
	// MappingWellKnownKindTimestamp maps google.protobuf.Timestamp.
	MappingWellKnownKindTimestamp MappingWellKnownKind = iota
	// MappingWellKnownKindDuration maps google.protobuf.Duration.
	MappingWellKnownKindDuration
)

// MappingWellKnownPlan carries non-dynamic well-known type mapping details.
type MappingWellKnownPlan struct {
	Kind MappingWellKnownKind
}

// MappingRefPlan refers to another mapping function, local or imported.
type MappingRefPlan struct {
	Name   string
	Ref    GoSymbolRef
	Source TypePlan
	Target TypePlan
}

// MappingEnumPlan describes enum conversion between protobuf and Tego enum types.
type MappingEnumPlan struct {
	Source TypePlan
	Target TypePlan
}

// MappingCastPlan describes scalar conversion that is rendered as a Go cast.
type MappingCastPlan struct {
	Source      TypePlan
	Target      TypePlan
	ProtoTarget bool
}

// ServicePlan describes generated service/client interfaces and transport adapters.
type ServicePlan struct {
	ProtoName             protoreflect.FullName
	ProtoRef              GoTypeRef
	ConnectRef            GoTypeRef
	Name                  string
	UnimplementedName     string
	GRPCServerName        string
	GRPCAdapterName       string
	GRPCClientName        string
	GRPCRegisterName      string
	GRPCNewServerName     string
	GRPCNewClientName     string
	ConnectHandlerName    string
	ConnectAdapterName    string
	ConnectClientName     string
	ConnectNewHandlerName string
	ConnectNewClientName  string
	Comment               string
	Methods               []ServiceMethodPlan
}

// ServiceMethodPlan describes one RPC method in generated Tego terms.
type ServiceMethodPlan struct {
	ProtoName      protoreflect.FullName
	ProtoGoName    string
	Name           string
	Comment        string
	Procedure      string
	StreamType     ServiceStreamType
	Request        ServiceMessagePlan
	Response       ServiceMessagePlan
	InlineRequest  *ServiceInlineHelperPlan
	InlineResponse *ServiceInlineHelperPlan
}

// ServiceMessagePlan describes an RPC request or response type and its conversions.
type ServiceMessagePlan struct {
	ProtoName protoreflect.FullName
	ProtoType TypePlan
	Type      TypePlan
	FromProto MappingValuePlan
	ToProto   MappingValuePlan
}

// ServiceInlineHelperPlan describes generated helpers that convert between wrapper request/response
// structs and inlined facade call shapes.
type ServiceInlineHelperPlan struct {
	ProtoName      protoreflect.FullName
	Type           TypePlan
	ToInlineName   string
	FromInlineName string
	Fields         []ServiceInlineFieldPlan
}

// ServiceInlineFieldPlan describes one field exposed by an inlined facade method.
type ServiceInlineFieldPlan struct {
	Name      string
	FieldName string
	Type      TypePlan
}

// ServiceStreamType classifies the streaming shape of an RPC method.
type ServiceStreamType uint

const (
	// ServiceStreamTypeUnary is a method with one request and one response.
	ServiceStreamTypeUnary ServiceStreamType = iota
	// ServiceStreamTypeClientStreaming is a method with a request stream and one response.
	ServiceStreamTypeClientStreaming
	// ServiceStreamTypeServerStreaming is a method with one request and a response stream.
	ServiceStreamTypeServerStreaming
	// ServiceStreamTypeBidiStreaming is a method with both request and response streams.
	ServiceStreamTypeBidiStreaming
)

// Diagnostic is a generalized type used for presenting helpful messages to users to help them find
// and fix issues found during planning.
type Diagnostic struct {
	Level   DiagnosticLevel
	Path    string
	Message string
}

// HasFatalDiagnostics returns whether the supplied diagnostics contain at least one fatal
// diagnostic.
func HasFatalDiagnostics(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == DiagnosticLevelFatal {
			return true
		}
	}
	return false
}

// String returns this Diagnostic as a string.
func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("%s: %s", d.Level, d.Message)
	}
	return fmt.Sprintf("%s: %s: %s", d.Level, d.Path, d.Message)
}

// DiagnosticLevel enumerates the possible levels of diagnostics, which can be used to determine
// whether a plan failed.
type DiagnosticLevel uint

const (
	// DiagnosticLevelFatal blocks code generation.
	DiagnosticLevelFatal DiagnosticLevel = iota
	// DiagnosticLevelWarning is reported to the user but does not block code generation.
	DiagnosticLevelWarning
	diagnosticLevelMax
)

var diagnosticLevelNames = map[DiagnosticLevel]string{
	DiagnosticLevelFatal:   "fatal",
	DiagnosticLevelWarning: "warning",
}

// String returns this DiagnosticLevel as a string.
func (d DiagnosticLevel) String() string {
	if s, ok := diagnosticLevelNames[d]; ok {
		return s
	}
	return "unknown"
}
