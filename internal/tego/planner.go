package tego

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/seeruk/tego/internal/types"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Planner is Tego's core planner, responsible for planning what and how Tego will generate Go code.
type Planner struct {
	typeLoader *types.Loader
	modulePath string
	rpc        RPCOptions
}

// PlannerOption represents a function that configures a Planner instance.
type PlannerOption func(*Planner)

// WithTypeLoader configures the loader used to resolve custom Go type options.
func WithTypeLoader(loader *types.Loader) PlannerOption {
	return func(planner *Planner) {
		planner.typeLoader = loader
	}
}

// WithModulePath configures the module path used when deriving generated output paths.
func WithModulePath(modulePath string) PlannerOption {
	return func(planner *Planner) {
		planner.modulePath = modulePath
	}
}

// WithRPCPlanning controls whether protobuf services are represented in the plan.
func WithRPCPlanning(options RPCOptions) PlannerOption {
	return func(planner *Planner) {
		planner.rpc = options
	}
}

// NewPlanner returns a new Planner instance.
func NewPlanner(opts ...PlannerOption) *Planner {
	planner := &Planner{
		typeLoader: types.NewLoader(),
		rpc:        defaultRPCOptions(),
	}
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

	if p.rpc.Enabled() {
		structsByProtoName := indexStructsByProtoName(plan.Structs)
		for _, service := range file.Services {
			p.planFileService(&plan, service, si, structsByProtoName)
		}
		requestInlineHelpers, responseInlineHelpers, diagnostics := plannedServiceInlineHelpers(plan.ProtoPath, plan.Services)
		plan.RequestInlineHelpers = requestInlineHelpers
		plan.ResponseInlineHelpers = responseInlineHelpers
		plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
	}

	plan.Diagnostics = append(plan.Diagnostics, plannedNameCollisionDiagnostics(plan, p.rpc)...)
	return plan
}

func indexStructsByProtoName(structs []StructPlan) map[protoreflect.FullName]StructPlan {
	index := make(map[protoreflect.FullName]StructPlan, len(structs))
	for _, structure := range structs {
		index[structure.ProtoName] = structure
	}
	return index
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

	plan.Diagnostics = append(plan.Diagnostics, flattenMessageDiagnostics(message)...)

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

func flattenMessageDiagnostics(message *ProtoMessage) []Diagnostic {
	if !message.Options.GetFlatten() {
		return nil
	}

	path := string(message.FullName)

	var diagnostics []Diagnostic
	if message.Options.HasInferShape() {
		diagnostics = append(diagnostics, warningDiagnostic(
			path,
			"infer_shape only controls automatic shape detection when flatten is not set",
		))
	}
	if message.Options.HasGoType() {
		diagnostics = append(diagnostics, fatalDiagnostic(
			path,
			"flatten conflicts with message-level go_type; use field-level go_type on the flattened field",
		))
	}
	if len(message.Fields) != 1 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "flatten message must have exactly one field"))
	}
	if len(message.Oneofs) > 0 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "flatten message must not declare oneofs"))
	}
	if len(message.Enums) > 0 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "flatten message must not declare nested enums"))
	}
	if len(message.Messages) > 0 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "flatten message must not declare nested messages"))
	}
	for _, field := range message.Fields {
		if field.Oneof != nil {
			diagnostics = append(diagnostics, fatalDiagnostic(path, "flatten message field must not be part of a oneof"))
			break
		}
	}

	return diagnostics
}

func (p *Planner) planFileOneof(plan *FilePlan, oneof *ProtoOneof, si *ShapeIndex) {
	oneofPlan, diagnostics := p.planOneof(oneof, si)
	plan.Oneofs = append(plan.Oneofs, oneofPlan)
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
}

func (p *Planner) planFileService(
	plan *FilePlan,
	service *ProtoService,
	si *ShapeIndex,
	structsByProtoName map[protoreflect.FullName]StructPlan,
) {
	servicePlan, diagnostics := p.planService(service, si, structsByProtoName)
	plan.Services = append(plan.Services, servicePlan)
	plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
}

func plannedServiceInlineHelpers(protoPath string, services []ServicePlan) ([]ServiceInlineHelperPlan, []ServiceInlineHelperPlan, []Diagnostic) {
	seen := make(map[string]serviceInlineHelperUse)
	var requestHelpers []ServiceInlineHelperPlan
	var responseHelpers []ServiceInlineHelperPlan
	var diagnostics []Diagnostic

	for _, service := range services {
		for _, method := range service.Methods {
			if method.InlineRequest != nil {
				added, helperDiagnostics := collectServiceInlineHelper(
					protoPath,
					seen,
					*method.InlineRequest,
					"request",
				)
				diagnostics = append(diagnostics, helperDiagnostics...)
				if added {
					requestHelpers = append(requestHelpers, *method.InlineRequest)
				}
			}
			if method.InlineResponse != nil {
				added, helperDiagnostics := collectServiceInlineHelper(
					protoPath,
					seen,
					*method.InlineResponse,
					"response",
				)
				diagnostics = append(diagnostics, helperDiagnostics...)
				if added {
					responseHelpers = append(responseHelpers, *method.InlineResponse)
				}
			}
		}
	}

	return requestHelpers, responseHelpers, diagnostics
}

type serviceInlineHelperUse struct {
	helper ServiceInlineHelperPlan
	owner  string
}

func collectServiceInlineHelper(
	protoPath string,
	seen map[string]serviceInlineHelperUse,
	helper ServiceInlineHelperPlan,
	helperKind string,
) (bool, []Diagnostic) {
	use := serviceInlineHelperUse{
		helper: helper,
		owner:  inlineHelperOwner(helper, helperKind),
	}

	for _, name := range []string{helper.ToInlineName, helper.FromInlineName} {
		known, exists := seen[name]
		if !exists {
			continue
		}
		if !serviceInlineHelperUsesEqual(known, use) {
			return false, []Diagnostic{incompatibleServiceInlineHelperDiagnostic(
				protoPath,
				name,
				known,
				use,
			)}
		}
		return false, nil
	}

	seen[helper.ToInlineName] = use
	seen[helper.FromInlineName] = use
	return true, nil
}

func serviceInlineHelperUsesEqual(a, b serviceInlineHelperUse) bool {
	return a.owner == b.owner && serviceInlineHelpersEqual(a.helper, b.helper)
}

func serviceInlineHelpersEqual(a, b ServiceInlineHelperPlan) bool {
	return a.ProtoName == b.ProtoName &&
		a.ToInlineName == b.ToInlineName &&
		a.FromInlineName == b.FromInlineName &&
		reflect.DeepEqual(a.Type, b.Type) &&
		reflect.DeepEqual(a.Fields, b.Fields)
}

func incompatibleServiceInlineHelperDiagnostic(
	protoPath string,
	name string,
	previous serviceInlineHelperUse,
	next serviceInlineHelperUse,
) Diagnostic {
	return fatalDiagnostic(
		protoPath,
		"planned Go name %q is used by incompatible inline helpers for %s and %s",
		name,
		previous.owner,
		next.owner,
	)
}

func inlineHelperOwner(helper ServiceInlineHelperPlan, helperKind string) string {
	return fmt.Sprintf("%s inline %s helper", helper.ProtoName, helperKind)
}

func plannedNameCollisionDiagnostics(plan FilePlan, rpc RPCOptions) []Diagnostic {
	inlineHelperCount := len(plan.RequestInlineHelpers) + len(plan.ResponseInlineHelpers)
	seen := make(map[string]string, len(plan.Enums)+len(plan.Oneofs)+len(plan.Structs)+len(plan.Services)*8+inlineHelperCount*2)
	var diagnostics []Diagnostic

	add := func(name, owner string) {
		diagnostics = append(diagnostics, plannedNameCollisionDiagnostic(plan, seen, name, owner)...)
	}
	addInlineHelper := func(helper ServiceInlineHelperPlan, helperKind string) {
		owner := inlineHelperOwner(helper, helperKind)
		add(helper.ToInlineName, owner)
		add(helper.FromInlineName, owner)
	}

	for _, enum := range plan.Enums {
		add(enum.Name, string(enum.ProtoName))
	}
	for _, oneof := range plan.Oneofs {
		add(oneof.Name, string(oneof.ProtoName))
		for _, variant := range oneof.Variants {
			add(variant.Name, string(variant.ProtoName))
		}
	}
	for _, structure := range plan.Structs {
		add(structure.Name, string(structure.ProtoName))
	}
	for _, helper := range plan.RequestInlineHelpers {
		addInlineHelper(helper, "request")
	}
	for _, helper := range plan.ResponseInlineHelpers {
		addInlineHelper(helper, "response")
	}
	for _, service := range plan.Services {
		add(service.Name, string(service.ProtoName))
		add(service.UnimplementedName, string(service.ProtoName)+" unimplemented facade")
		if rpc.GRPC {
			add(service.GRPCAdapterName, string(service.ProtoName)+" gRPC adapter")
			add(service.GRPCRegisterName, string(service.ProtoName)+" gRPC server registration helper")
			add(service.GRPCNewServerName, string(service.ProtoName)+" gRPC server constructor")
			add(service.GRPCNewClientName, string(service.ProtoName)+" gRPC client constructor")
		}
		if rpc.Connect {
			add(service.ConnectAdapterName, string(service.ProtoName)+" Connect adapter")
			add(service.ConnectNewHandlerName, string(service.ProtoName)+" Connect handler constructor")
			add(service.ConnectNewClientName, string(service.ProtoName)+" Connect client constructor")
		}
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
