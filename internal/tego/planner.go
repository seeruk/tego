package tego

import (
	"fmt"
	"path"
	"strings"

	"github.com/seeruk/tego/internal/types"
	"google.golang.org/protobuf/compiler/protogen"
)

// Planner is Tego's core planner, responsible for planning what and how Tego will generate Go code.
type Planner struct {
	typeLoader *types.Loader
	modulePath string
}

type PlannerOption func(*Planner)

func WithTypeLoader(loader *types.Loader) PlannerOption {
	return func(planner *Planner) {
		planner.typeLoader = loader
	}
}

func WithModulePath(modulePath string) PlannerOption {
	return func(planner *Planner) {
		planner.modulePath = modulePath
	}
}

// NewPlanner returns a new Planner instance.
func NewPlanner(opts ...PlannerOption) *Planner {
	planner := &Planner{typeLoader: types.NewLoader()}
	for _, opt := range opts {
		opt(planner)
	}
	return planner
}

// Plan attempts to build a plan using the given DescriptorIndex and ShapeIndex. If successfully,
// the returned Plan should be ready to generate code for.
func (p *Planner) Plan(di *DescriptorIndex, si *ShapeIndex) (Plan, error) {
	var plan Plan

	for _, file := range di.Files {
		if !file.Generate {
			continue
		}

		plan.Files = append(plan.Files, p.planFile(file, si))
	}

	propagateMappingErrors(&plan)

	return plan, nil
}

func (p *Planner) planFile(file *ProtoFile, si *ShapeIndex) FilePlan {
	plan := FilePlan{
		ProtoPath: file.Path,
	}

	if !file.Options.HasGoPackage() {
		plan.Diagnostics = append(plan.Diagnostics, fatalDiagnostic(file.Path, "missing required tego.file go_package option"))
		return plan
	}

	plan.Package = packageRef(file.Options.GetGoPackage())

	output, diagnostics := p.planFileOutput(file, plan.Package)
	plan.Output = output
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)

	for _, enum := range file.Enums {
		p.planFileEnum(&plan, enum)
	}

	for _, message := range file.Messages {
		p.planFileMessage(&plan, message, si)
	}

	plan.Diagnostics = append(plan.Diagnostics, plannedNameCollisionDiagnostics(plan)...)
	return plan
}

func (p *Planner) planFileEnum(plan *FilePlan, enum *ProtoEnum) {
	enumPlan, diagnostics, ok := p.planEnum(enum)
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
	if ok {
		plan.Enums = append(plan.Enums, enumPlan)
	}
}

func (p *Planner) planFileMessage(plan *FilePlan, message *ProtoMessage, si *ShapeIndex) {
	for _, enum := range message.Enums {
		p.planFileEnum(plan, enum)
	}

	structPlan, diagnostics, ok := p.planStruct(message, si)
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
	if ok {
		for _, oneof := range message.Oneofs {
			p.planFileOneof(plan, oneof, si)
		}
		plan.Structs = append(plan.Structs, structPlan)
		plan.Mappings = append(plan.Mappings, p.planMapping(message, structPlan, si))
	}

	for _, nested := range message.Messages {
		p.planFileMessage(plan, nested, si)
	}
}

func (p *Planner) planFileOneof(plan *FilePlan, oneof *ProtoOneof, si *ShapeIndex) {
	oneofPlan, diagnostics := p.planOneof(oneof, si)
	plan.Oneofs = append(plan.Oneofs, oneofPlan)
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
}

func plannedNameCollisionDiagnostics(plan FilePlan) []Diagnostic {
	seen := make(map[string]string, len(plan.Enums)+len(plan.Oneofs)+len(plan.Structs))
	var diagnostics []Diagnostic

	for _, enum := range plan.Enums {
		diagnostics = append(diagnostics, plannedNameCollisionDiagnostic(plan, seen, enum.Name, string(enum.ProtoName))...)
	}
	for _, oneof := range plan.Oneofs {
		diagnostics = append(diagnostics, plannedNameCollisionDiagnostic(plan, seen, oneof.Name, string(oneof.ProtoName))...)
		for _, variant := range oneof.Variants {
			diagnostics = append(diagnostics, plannedNameCollisionDiagnostic(plan, seen, variant.Name, string(variant.ProtoName))...)
		}
	}
	for _, structure := range plan.Structs {
		diagnostics = append(diagnostics, plannedNameCollisionDiagnostic(plan, seen, structure.Name, string(structure.ProtoName))...)
	}

	return diagnostics
}

func plannedNameCollisionDiagnostic(plan FilePlan, seen map[string]string, name, owner string) []Diagnostic {
	if previous, ok := seen[name]; ok {
		return []Diagnostic{fatalDiagnostic(
			plan.ProtoPath,
			"planned Go name %q is used by both %s and %s",
			name,
			previous,
			owner,
		)}
	}

	seen[name] = owner
	return nil
}

func packageRef(goPackage string) PackageRef {
	importPath, name, ok := strings.Cut(goPackage, ";")
	if !ok {
		importPath = goPackage
		name = path.Base(importPath)
	}

	return PackageRef{
		ImportPath: importPath,
		Name:       name,
	}
}

func (p *Planner) planFileOutput(file *ProtoFile, pkg PackageRef) (FileOutputPlan, []Diagnostic) {
	directory := pkg.ImportPath
	if p.modulePath != "" {
		var ok bool
		directory, ok = stripModulePrefix(directory, p.modulePath)
		if !ok {
			return FileOutputPlan{}, []Diagnostic{moduleMismatchDiagnostic(file.Path, pkg.ImportPath, p.modulePath)}
		}
	}

	if outputPath := file.Options.GetOutputPath(); outputPath != "" {
		return p.explicitFileOutput(file.Path, outputPath)
	}

	filename := generatedFilename(file.Path)
	generatorPath := path.Join(pkg.ImportPath, filename)

	return FileOutputPlan{
		Directory:     directory,
		Filename:      filename,
		Path:          joinPath(directory, filename),
		GeneratorPath: generatorPath,
	}, nil
}

func (p *Planner) explicitFileOutput(protoPath, outputPath string) (FileOutputPlan, []Diagnostic) {
	clean, diagnostics := validateOutputPath(protoPath, outputPath)
	if len(diagnostics) > 0 {
		return FileOutputPlan{}, diagnostics
	}

	directory := path.Dir(clean)
	if directory == "." {
		directory = ""
	}

	generatorPath := clean
	if p.modulePath != "" {
		generatorPath = path.Join(p.modulePath, clean)
	}

	return FileOutputPlan{
		Directory:     directory,
		Filename:      path.Base(clean),
		Path:          clean,
		GeneratorPath: generatorPath,
	}, nil
}

func generatedFilename(protoPath string) string {
	name := path.Base(protoPath)
	ext := path.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return name + ".tego.go"
}

func validateOutputPath(protoPath, outputPath string) (string, []Diagnostic) {
	clean := path.Clean(outputPath)
	if outputPath == "" || clean == "." {
		return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path must include a filename")}
	}
	if path.IsAbs(outputPath) {
		return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path must be relative")}
	}
	for _, part := range strings.Split(outputPath, "/") {
		if part == ".." {
			return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path must not contain parent traversal")}
		}
	}
	if strings.HasSuffix(outputPath, "/") {
		return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path must include a filename")}
	}
	filename := path.Base(clean)
	if filename == "." || filename == "/" || filename == "" {
		return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path must include a filename")}
	}
	if !strings.HasSuffix(filename, ".go") {
		return "", []Diagnostic{fatalDiagnostic(protoPath, "tego.file output_path filename must end in .go")}
	}
	return clean, nil
}

func stripModulePrefix(importPath, modulePath string) (string, bool) {
	if importPath == modulePath {
		return "", true
	}
	return strings.CutPrefix(importPath, modulePath+"/")
}

func moduleMismatchDiagnostic(protoPath, importPath, modulePath string) Diagnostic {
	return fatalDiagnostic(
		protoPath,
		"tego.file go_package %q is outside module %q",
		importPath,
		modulePath,
	)
}

func joinPath(directory, filename string) string {
	if directory == "" {
		return filename
	}
	return path.Join(directory, filename)
}

func plannedComment(explicit string, hasExplicit bool, source protogen.Comments, protoName, plannedName string) string {
	if hasExplicit {
		return explicit
	}

	comment := strings.TrimSpace(string(source))
	if !strings.HasPrefix(comment, protoName) {
		return ""
	}

	return plannedName + strings.TrimPrefix(comment, protoName)
}

func fatalDiagnostic(path, format string, args ...any) Diagnostic {
	return Diagnostic{
		Level:   DiagnosticLevelFatal,
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}

func warningDiagnostic(path, format string, args ...any) Diagnostic {
	return Diagnostic{
		Level:   DiagnosticLevelWarning,
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}
