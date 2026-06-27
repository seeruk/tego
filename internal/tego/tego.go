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
	OutputPath  string
	Package     PackageRef
	Enums       []EnumPlan
	Structs     []StructPlan
	Diagnostics []Diagnostic
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
	TypeKindPointer
	TypeKindSlice
	TypeKindMap
	TypeKindOmittable
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
)

type GoTypeRef struct {
	ImportPath string
	Name       string
}

type CustomGoTypePlan struct {
	Ref       GoTypeRef
	FromProto GoSymbolRef
	ToProto   GoSymbolRef
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

// Diagnostic is a generalized type used for presenting helpful messages to Morph consumers to help
// them find and fix issues found during planning.
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
