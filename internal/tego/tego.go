package tego

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type Plan struct {
	Files []FilePlan
}

type FilePlan struct {
	ProtoPath   string
	Output      FileOutputPlan
	Package     PackageRef
	Enums       []EnumPlan
	Oneofs      []OneofPlan
	Structs     []StructPlan
	Mappings    []MappingPlan
	Diagnostics []Diagnostic
}

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

type PackageRef struct {
	ImportPath string
	Name       string
}

type EnumPlan struct {
	ProtoName  protoreflect.FullName
	Name       string
	Comment    string
	Underlying EnumUnderlyingType
	Constants  []EnumConstantPlan
}

type EnumUnderlyingType uint

const (
	EnumUnderlyingTypeUint EnumUnderlyingType = iota
	EnumUnderlyingTypeInt
	EnumUnderlyingTypeString
)

type EnumConstantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Value     EnumConstantValue
}

type EnumConstantValue struct {
	Uint   uint
	Int    int
	String string
}

type OneofPlan struct {
	ProtoName    protoreflect.FullName
	Name         string
	Comment      string
	MarkerMethod string
	Variants     []OneofVariantPlan
}

type OneofVariantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	FieldName string
	Comment   string
	Type      TypePlan
}

type StructPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Fields    []FieldPlan
}

type FieldPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Comment   string
	Type      TypePlan
	Tags      []StructTagPlan
}

type TypePlan struct {
	Kind   TypeKind
	Scalar ScalarKind
	Ref    GoTypeRef
	Custom CustomGoTypePlan
	Elem   *TypePlan
	Key    *TypePlan
	Value  *TypePlan
}

type TypeKind uint

const (
	TypeKindScalar TypeKind = iota
	TypeKindEnum
	TypeKindStruct
	TypeKindExternal
	TypeKindCustom
	TypeKindOneof
	TypeKindPointer
	TypeKindSlice
	TypeKindMap
	TypeKindOmittable
	TypeKindEmptyStruct
)

type ScalarKind uint

const (
	ScalarKindBool    ScalarKind = iota
	ScalarKindInt32              // from int32, sint32, sfixed32
	ScalarKindInt64              // from int64, sint64, sfixed64
	ScalarKindUint32             // from uint32, fixed32
	ScalarKindUint64             // from uint64, fixed64
	ScalarKindFloat32            // from float
	ScalarKindFloat64            // from double
	ScalarKindString             // from string
	ScalarKindBytes              // from bytes
	ScalarKindAny                // from well-known dynamic values
)

type GoTypeRef struct {
	ImportPath string
	Name       string
	Args       []GoTypeRef
	Pointer    *GoTypeRef
	Slice      *GoTypeRef
}

type CustomGoTypePlan struct {
	Ref               GoTypeRef
	FromProto         GoSymbolRef
	FromProtoCanError bool
	ToProto           GoSymbolRef
	ToProtoCanError   bool
}

type GoSymbolRef struct {
	ImportPath string
	Name       string
	Receiver   string
}

type StructTagPlan struct {
	Key   string
	Value string
}

type MappingPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	ProtoRef  GoTypeRef
	Type      TypePlan
	FromProto MappingFunctionPlan
	ToProto   MappingFunctionPlan
	Fields    []FieldMappingPlan
}

type MappingFunctionPlan struct {
	Name         string
	ReceiverName string
	Source       TypePlan
	Target       TypePlan
	CanError     bool
}

type FieldMappingPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	Proto     MappingFieldAccessPlan
	FromProto MappingValuePlan
	ToProto   MappingValuePlan
}

type MappingValuePlan struct {
	Kind     MappingValueKind
	Source   TypePlan
	Target   TypePlan
	CanError bool
	Access   MappingAccessPlan
	Oneof    *MappingOneofPlan
	Struct   *MappingRefPlan
	Custom   *CustomGoTypePlan
	Enum     *MappingEnumPlan
	Cast     *MappingCastPlan
	Dynamic  *MappingDynamicPlan
	Elem     *MappingValuePlan
	Key      *MappingValuePlan
	Value    *MappingValuePlan
}

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

type MappingFieldAccessPlan struct {
	Name   string
	Getter string
	Setter string
	Has    string
	Clear  string
}

type MappingOneofAccessPlan struct {
	Name     string
	Which    string
	Value    MappingFieldAccessPlan
	Null     MappingFieldAccessPlan
	ValueRef GoTypeRef
	NullRef  GoTypeRef
}

type MappingOneofPlan struct {
	Which    string
	Variants []MappingOneofVariantPlan
}

type MappingOneofVariantPlan struct {
	ProtoName protoreflect.FullName
	Name      string
	FieldName string
	Proto     MappingFieldAccessPlan
	Case      GoTypeRef
	Value     MappingValuePlan
}

type MappingNullableForm uint

const (
	MappingNullableFormNone MappingNullableForm = iota
	MappingNullableFormPointer
	MappingNullableFormOneof
	MappingNullableFormValue
)

type MappingValueKind uint

const (
	MappingValueKindUnsupported MappingValueKind = iota
	MappingValueKindDirect
	MappingValueKindScalarCast
	MappingValueKindEnum
	MappingValueKindStruct
	MappingValueKindCustom
	MappingValueKindNullable
	MappingValueKindSlice
	MappingValueKindMap
	MappingValueKindOmittable
	MappingValueKindOneof
	MappingValueKindEmptyStruct
	MappingValueKindDynamic
	MappingValueKindFlatten
)

type MappingDynamicKind uint

const (
	MappingDynamicKindStruct MappingDynamicKind = iota
	MappingDynamicKindValue
	MappingDynamicKindListValue
)

type MappingDynamicPlan struct {
	Kind MappingDynamicKind
}

type MappingRefPlan struct {
	Name   string
	Source TypePlan
	Target TypePlan
}

type MappingEnumPlan struct {
	Source TypePlan
	Target TypePlan
}

type MappingCastPlan struct {
	Source TypePlan
	Target TypePlan
}

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
	DiagnosticLevelFatal DiagnosticLevel = iota
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
