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

type StructPlan struct{}

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
